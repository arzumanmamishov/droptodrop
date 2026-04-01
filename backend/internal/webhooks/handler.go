package webhooks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/droptodrop/droptodrop/internal/audit"
	"github.com/droptodrop/droptodrop/internal/orders"
	"github.com/droptodrop/droptodrop/internal/queue"
	"github.com/droptodrop/droptodrop/internal/shops"
	verifyhmac "github.com/droptodrop/droptodrop/pkg/hmac"
)

// Handler processes Shopify webhooks with HMAC verification.
type Handler struct {
	db        *pgxpool.Pool
	queue     *queue.Client
	shopsSvc  *shops.Service
	ordersSvc *orders.Service
	secret    string
	logger    zerolog.Logger
	audit     *audit.Service
}

// NewHandler creates a webhook handler.
func NewHandler(db *pgxpool.Pool, q *queue.Client, shopsSvc *shops.Service, ordersSvc *orders.Service, secret string, logger zerolog.Logger, auditSvc *audit.Service) *Handler {
	return &Handler{
		db:        db,
		queue:     q,
		shopsSvc:  shopsSvc,
		ordersSvc: ordersSvc,
		secret:    secret,
		logger:    logger,
		audit:     auditSvc,
	}
}

// verifyAndExtract reads the raw body, verifies HMAC, and returns the raw bytes and parsed payload.
func (h *Handler) verifyAndExtract(c *gin.Context) ([]byte, map[string]interface{}, error) {
	// Read raw body BEFORE any JSON parsing
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read body: %w", err)
	}
	// Restore body for any downstream use
	c.Request.Body = io.NopCloser(bytes.NewReader(rawBody))

	// Verify HMAC using raw body bytes
	hmacHeader := c.GetHeader("X-Shopify-Hmac-Sha256")
	if !verifyhmac.VerifyWebhook(rawBody, h.secret, hmacHeader) {
		return nil, nil, fmt.Errorf("HMAC verification failed")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, nil, fmt.Errorf("parse payload: %w", err)
	}

	return rawBody, payload, nil
}

// recordWebhookEvent stores the webhook in the database for idempotency and audit.
func (h *Handler) recordWebhookEvent(c *gin.Context, topic string, rawBody []byte) (string, bool) {
	shopDomain := c.GetHeader("X-Shopify-Shop-Domain")
	webhookID := c.GetHeader("X-Shopify-Webhook-Id")

	// Compute payload hash for deduplication
	hash := fmt.Sprintf("%x", sha256.Sum256(rawBody))

	// Check for duplicate using Shopify's webhook ID (unique per delivery)
	var existingID string
	if webhookID != "" {
		err := h.db.QueryRow(c.Request.Context(), `
			SELECT id FROM webhook_events WHERE shopify_webhook_id = $1 AND status IN ('processed', 'processing')
		`, webhookID).Scan(&existingID)
		if err == nil {
			h.logger.Info().Str("topic", topic).Str("webhook_id", webhookID).Msg("duplicate webhook, skipping")
			return existingID, true
		}
	}

	// Get shop ID
	var shopID *string
	_ = h.db.QueryRow(c.Request.Context(), `SELECT id FROM shops WHERE shopify_domain = $1`, shopDomain).Scan(&shopID)

	// Insert event record
	var eventID string
	err := h.db.QueryRow(c.Request.Context(), `
		INSERT INTO webhook_events (shop_id, topic, shopify_webhook_id, payload_hash, status)
		VALUES ($1, $2, $3, $4, 'processing')
		RETURNING id
	`, shopID, topic, webhookID, hash).Scan(&eventID)
	if err != nil {
		h.logger.Error().Err(err).Str("topic", topic).Msg("failed to record webhook event")
		return "", false
	}

	return eventID, false
}

// markWebhookProcessed updates the webhook event status.
func (h *Handler) markWebhookProcessed(c *gin.Context, eventID, status, errorMsg string) {
	_, err := h.db.Exec(c.Request.Context(), `
		UPDATE webhook_events SET status = $2, error_message = $3, processed_at = NOW(), attempts = attempts + 1
		WHERE id = $1
	`, eventID, status, errorMsg)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", eventID).Msg("failed to update webhook status")
	}
}

// AppUninstalled handles the app/uninstalled webhook.
func (h *Handler) AppUninstalled(c *gin.Context) {
	rawBody, _, err := h.verifyAndExtract(c)
	if err != nil {
		h.logger.Warn().Err(err).Msg("webhook verification failed: app/uninstalled")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	eventID, isDuplicate := h.recordWebhookEvent(c, "app/uninstalled", rawBody)
	if isDuplicate {
		c.JSON(http.StatusOK, gin.H{"status": "already_processed"})
		return
	}

	shopDomain := c.GetHeader("X-Shopify-Shop-Domain")
	if err := h.shopsSvc.Deactivate(c.Request.Context(), shopDomain); err != nil {
		h.logger.Error().Err(err).Str("shop", shopDomain).Msg("failed to deactivate shop")
		h.markWebhookProcessed(c, eventID, "failed", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "processing failed"})
		return
	}

	h.markWebhookProcessed(c, eventID, "processed", "")
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// OrdersCreate handles the orders/create webhook.
func (h *Handler) OrdersCreate(c *gin.Context) {
	rawBody, payload, err := h.verifyAndExtract(c)
	if err != nil {
		h.logger.Warn().Err(err).Msg("webhook verification failed: orders/create")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	eventID, isDuplicate := h.recordWebhookEvent(c, "orders/create", rawBody)
	if isDuplicate {
		c.JSON(http.StatusOK, gin.H{"status": "already_processed"})
		return
	}

	shopDomain := c.GetHeader("X-Shopify-Shop-Domain")

	// Look up shop
	var shopID string
	err = h.db.QueryRow(c.Request.Context(), `SELECT id FROM shops WHERE shopify_domain = $1 AND role = 'reseller'`, shopDomain).Scan(&shopID)
	if err != nil {
		h.logger.Info().Str("shop", shopDomain).Msg("order webhook from non-reseller, skipping")
		h.markWebhookProcessed(c, eventID, "skipped", "not a reseller shop")
		c.JSON(http.StatusOK, gin.H{"status": "skipped"})
		return
	}

	// Process order routing in background goroutine
	go func() {
		bgCtx := context.Background()
		if err := h.ordersSvc.RouteOrder(bgCtx, shopID, payload); err != nil {
			h.logger.Error().Err(err).Msg("order routing failed")
		} else {
			h.logger.Info().Str("shop_id", shopID).Msg("order routed successfully")
		}
	}()

	h.markWebhookProcessed(c, eventID, "processed", "")
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ProductsUpdate handles the products/update webhook.
func (h *Handler) ProductsUpdate(c *gin.Context) {
	rawBody, payload, err := h.verifyAndExtract(c)
	if err != nil {
		h.logger.Warn().Err(err).Msg("webhook verification failed: products/update")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	eventID, isDuplicate := h.recordWebhookEvent(c, "products/update", rawBody)
	if isDuplicate {
		c.JSON(http.StatusOK, gin.H{"status": "already_processed"})
		return
	}

	// Queue product sync
	shopDomain := c.GetHeader("X-Shopify-Shop-Domain")
	_, err = h.queue.Enqueue(c.Request.Context(), "products", "sync_product_update", map[string]interface{}{
		"shop_domain": shopDomain,
		"product":     payload,
	}, 3)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to enqueue product update")
		h.markWebhookProcessed(c, eventID, "failed", err.Error())
	} else {
		h.markWebhookProcessed(c, eventID, "processed", "")
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ProductsDelete handles the products/delete webhook.
func (h *Handler) ProductsDelete(c *gin.Context) {
	rawBody, payload, err := h.verifyAndExtract(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	eventID, isDuplicate := h.recordWebhookEvent(c, "products/delete", rawBody)
	if isDuplicate {
		c.JSON(http.StatusOK, gin.H{"status": "already_processed"})
		return
	}

	productID, _ := payload["id"].(float64)
	shopDomain := c.GetHeader("X-Shopify-Shop-Domain")
	ctx := c.Request.Context()

	// Get the listing ID before deleting
	var listingID, supplierShopID string
	h.db.QueryRow(ctx, `
		SELECT sl.id, sl.supplier_shop_id FROM supplier_listings sl
		JOIN shops s ON s.id = sl.supplier_shop_id
		WHERE sl.shopify_product_id = $1 AND s.shopify_domain = $2
	`, int64(productID), shopDomain).Scan(&listingID, &supplierShopID)

	if listingID != "" {
		// Mark all reseller imports of this listing as removed
		h.db.Exec(ctx, `
			UPDATE reseller_imports SET status = 'removed', last_sync_error = 'Supplier deleted this product'
			WHERE supplier_listing_id = $1 AND status != 'removed'
		`, listingID)

		// Deactivate product links
		h.db.Exec(ctx, `
			UPDATE product_links SET is_active = FALSE
			WHERE supplier_product_id = $1 AND supplier_shop_id = $2
		`, int64(productID), supplierShopID)

		// Delete the listing (cascades to variants)
		_, err = h.db.Exec(ctx, `DELETE FROM supplier_listings WHERE id = $1`, listingID)
		if err != nil {
			h.logger.Error().Err(err).Msg("failed to delete listing")
			h.markWebhookProcessed(c, eventID, "failed", err.Error())
		} else {
			h.logger.Info().Str("listing_id", listingID).Int64("product_id", int64(productID)).Msg("listing deleted via webhook")
			h.markWebhookProcessed(c, eventID, "processed", "")

			if h.audit != nil {
				h.audit.Log(ctx, supplierShopID, "webhook", "", "listing_deleted_by_shopify", "supplier_listing", listingID, map[string]int64{"shopify_product_id": int64(productID)}, "success", "")
			}
		}
	} else {
		h.markWebhookProcessed(c, eventID, "processed", "listing not found")
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// InventoryUpdate handles the inventory_levels/update webhook.
func (h *Handler) InventoryUpdate(c *gin.Context) {
	rawBody, payload, err := h.verifyAndExtract(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	eventID, isDuplicate := h.recordWebhookEvent(c, "inventory_levels/update", rawBody)
	if isDuplicate {
		c.JSON(http.StatusOK, gin.H{"status": "already_processed"})
		return
	}

	// Queue inventory sync
	_, err = h.queue.Enqueue(c.Request.Context(), "inventory", "sync_inventory", map[string]interface{}{
		"shop_domain":     c.GetHeader("X-Shopify-Shop-Domain"),
		"inventory_level": payload,
	}, 3)
	if err != nil {
		h.markWebhookProcessed(c, eventID, "failed", err.Error())
	} else {
		h.markWebhookProcessed(c, eventID, "processed", "")
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// FulfillmentsCreate handles the fulfillments/create webhook.
func (h *Handler) FulfillmentsCreate(c *gin.Context) {
	rawBody, payload, err := h.verifyAndExtract(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	eventID, isDuplicate := h.recordWebhookEvent(c, "fulfillments/create", rawBody)
	if isDuplicate {
		c.JSON(http.StatusOK, gin.H{"status": "already_processed"})
		return
	}

	_, err = h.queue.Enqueue(c.Request.Context(), "fulfillments", "process_fulfillment", map[string]interface{}{
		"shop_domain": c.GetHeader("X-Shopify-Shop-Domain"),
		"fulfillment": payload,
	}, 3)
	if err != nil {
		h.markWebhookProcessed(c, eventID, "failed", err.Error())
	} else {
		h.markWebhookProcessed(c, eventID, "processed", "")
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
