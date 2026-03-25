package billing

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Handler provides billing-related endpoints.
// Billing is structured for future Shopify Billing API or Managed Pricing integration.
type Handler struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
}

// NewHandler creates a billing handler.
func NewHandler(db *pgxpool.Pool, logger zerolog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// Status represents the current billing state for a shop.
type Status struct {
	Plan     string `json:"plan"`
	Status   string `json:"status"`
	Features []string `json:"features"`
}

// GetStatus returns the billing status for a shop.
// This is a placeholder that can be wired to Shopify Billing API.
func (h *Handler) GetStatus(c *gin.Context) {
	shopID, _ := c.Get("shop_id")

	var plan, status string
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT COALESCE(billing_plan, 'free'), COALESCE(billing_status, 'inactive')
		FROM app_settings WHERE shop_id = $1
	`, shopID).Scan(&plan, &status)
	if err != nil {
		plan = "free"
		status = "inactive"
	}

	features := []string{"supplier_mode", "reseller_mode", "product_sync", "order_routing"}
	if plan != "free" {
		features = append(features, "bulk_import", "advanced_analytics", "priority_support")
	}

	c.JSON(http.StatusOK, Status{
		Plan:     plan,
		Status:   status,
		Features: features,
	})
}

// Note: To enable Shopify Billing API integration:
// 1. Use the appSubscriptionCreate GraphQL mutation
// 2. Handle the confirmation callback
// 3. Store the charge ID and plan details in app_settings
// 4. Verify subscription status on each authenticated request
// See: https://shopify.dev/docs/apps/billing
