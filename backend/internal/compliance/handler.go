package compliance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/droptodrop/droptodrop/internal/audit"
	verifyhmac "github.com/droptodrop/droptodrop/pkg/hmac"
)

// Handler processes mandatory Shopify compliance webhooks.
type Handler struct {
	db     *pgxpool.Pool
	secret string
	logger zerolog.Logger
	audit  *audit.Service
}

// NewHandler creates a compliance handler.
func NewHandler(db *pgxpool.Pool, secret string, logger zerolog.Logger, auditSvc *audit.Service) *Handler {
	return &Handler{db: db, secret: secret, logger: logger, audit: auditSvc}
}

func (h *Handler) verifyAndParse(c *gin.Context) (json.RawMessage, error) {
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(rawBody))

	hmacHeader := c.GetHeader("X-Shopify-Hmac-Sha256")
	if !verifyhmac.VerifyWebhook(rawBody, h.secret, hmacHeader) {
		return nil, fmt.Errorf("HMAC verification failed")
	}

	return rawBody, nil
}

// CustomersDataRequest handles the customers/data_request webhook.
// Shopify sends this when a customer requests their data.
func (h *Handler) CustomersDataRequest(c *gin.Context) {
	rawBody, err := h.verifyAndParse(c)
	if err != nil {
		h.logger.Warn().Err(err).Msg("compliance webhook verification failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var payload struct {
		ShopID         int64  `json:"shop_id"`
		ShopDomain     string `json:"shop_domain"`
		Customer       struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
			Phone string `json:"phone"`
		} `json:"customer"`
		OrdersRequested []int64 `json:"orders_requested"`
	}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	// Record compliance event
	var shopID *string
	_ = h.db.QueryRow(c.Request.Context(), `SELECT id FROM shops WHERE shopify_domain = $1`, payload.ShopDomain).Scan(&shopID)

	_, err = h.db.Exec(c.Request.Context(), `
		INSERT INTO compliance_events (shop_id, event_type, payload, status)
		VALUES ($1, 'customers_data_request', $2, 'received')
	`, shopID, rawBody)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to record compliance event")
	}

	h.logger.Info().Str("shop", payload.ShopDomain).Int64("customer_id", payload.Customer.ID).Msg("customers/data_request received")

	if h.audit != nil && shopID != nil {
		h.audit.Log(c.Request.Context(), *shopID, "webhook", "", "customers_data_request", "customer",
			fmt.Sprintf("%d", payload.Customer.ID), nil, "success", "")
	}

	// This app stores minimal customer data (only shipping info in routed_orders).
	// In production, you would compile and return the data.
	c.JSON(http.StatusOK, gin.H{"status": "acknowledged"})
}

// CustomersRedact handles the customers/redact webhook.
// Shopify sends this when a store owner requests deletion of customer data.
func (h *Handler) CustomersRedact(c *gin.Context) {
	rawBody, err := h.verifyAndParse(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var payload struct {
		ShopID     int64  `json:"shop_id"`
		ShopDomain string `json:"shop_domain"`
		Customer   struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
			Phone string `json:"phone"`
		} `json:"customer"`
		OrdersToRedact []int64 `json:"orders_to_redact"`
	}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	var shopID *string
	_ = h.db.QueryRow(c.Request.Context(), `SELECT id FROM shops WHERE shopify_domain = $1`, payload.ShopDomain).Scan(&shopID)

	_, err = h.db.Exec(c.Request.Context(), `
		INSERT INTO compliance_events (shop_id, event_type, payload, status)
		VALUES ($1, 'customers_redact', $2, 'processing')
	`, shopID, rawBody)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to record compliance event")
	}

	// Redact customer data from routed orders
	if shopID != nil {
		for _, orderID := range payload.OrdersToRedact {
			_, err := h.db.Exec(c.Request.Context(), `
				UPDATE routed_orders SET
					customer_shipping_name = '[REDACTED]',
					customer_shipping_address = '{}',
					customer_email = '[REDACTED]',
					customer_phone = '[REDACTED]'
				WHERE reseller_order_id = $1 AND reseller_shop_id = $2
			`, orderID, *shopID)
			if err != nil {
				h.logger.Error().Err(err).Int64("order_id", orderID).Msg("failed to redact customer data")
			}
		}
	}

	h.logger.Info().Str("shop", payload.ShopDomain).Msg("customers/redact processed")

	if h.audit != nil && shopID != nil {
		h.audit.Log(c.Request.Context(), *shopID, "webhook", "", "customers_redact", "customer",
			fmt.Sprintf("%d", payload.Customer.ID), nil, "success", "")
	}

	c.JSON(http.StatusOK, gin.H{"status": "acknowledged"})
}

// ShopRedact handles the shop/redact webhook.
// Shopify sends this 48 hours after an app is uninstalled.
func (h *Handler) ShopRedact(c *gin.Context) {
	rawBody, err := h.verifyAndParse(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var payload struct {
		ShopID     int64  `json:"shop_id"`
		ShopDomain string `json:"shop_domain"`
	}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	var shopID *string
	_ = h.db.QueryRow(c.Request.Context(), `SELECT id FROM shops WHERE shopify_domain = $1`, payload.ShopDomain).Scan(&shopID)

	_, err = h.db.Exec(c.Request.Context(), `
		INSERT INTO compliance_events (shop_id, event_type, payload, status)
		VALUES ($1, 'shop_redact', $2, 'processing')
	`, shopID, rawBody)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to record compliance event")
	}

	// Redact shop data - remove PII, keep structure for audit
	if shopID != nil {
		_, _ = h.db.Exec(c.Request.Context(), `UPDATE shops SET name = '[REDACTED]', email = '[REDACTED]' WHERE id = $1`, *shopID)
		_, _ = h.db.Exec(c.Request.Context(), `
			UPDATE routed_orders SET customer_shipping_name = '[REDACTED]', customer_shipping_address = '{}',
				customer_email = '[REDACTED]', customer_phone = '[REDACTED]'
			WHERE reseller_shop_id = $1 OR supplier_shop_id = $1
		`, *shopID)
	}

	h.logger.Info().Str("shop", payload.ShopDomain).Msg("shop/redact processed")
	c.JSON(http.StatusOK, gin.H{"status": "acknowledged"})
}
