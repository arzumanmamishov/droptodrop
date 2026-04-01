package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/droptodrop/droptodrop/internal/audit"
	"github.com/droptodrop/droptodrop/internal/config"
	"github.com/droptodrop/droptodrop/pkg/shopify"
)

// Handler manages OAuth install and callback flows.
type Handler struct {
	db     *pgxpool.Pool
	cfg    config.ShopifyConfig
	sessCfg config.SessionConfig
	encKey string
	logger zerolog.Logger
	audit  *audit.Service
}

// NewHandler creates an auth handler.
func NewHandler(db *pgxpool.Pool, cfg config.ShopifyConfig, sessCfg config.SessionConfig, encKey string, logger zerolog.Logger, auditSvc *audit.Service) *Handler {
	return &Handler{
		db:      db,
		cfg:     cfg,
		sessCfg: sessCfg,
		encKey:  encKey,
		logger:  logger,
		audit:   auditSvc,
	}
}

// Install initiates the Shopify OAuth flow.
func (h *Handler) Install(c *gin.Context) {
	shop := c.Query("shop")
	if !shopify.ValidateShopDomain(shop) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid shop domain"})
		return
	}

	// Generate CSRF nonce
	nonce, err := generateNonce()
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to generate nonce")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Store nonce temporarily (use DB or Redis in production)
	_, err = h.db.Exec(c.Request.Context(), `
		INSERT INTO shop_sessions (id, shop_id, session_token, expires_at)
		SELECT $1, s.id, $2, $3
		FROM shops s WHERE s.shopify_domain = $4
		ON CONFLICT DO NOTHING
	`, uuid.New(), "nonce:"+nonce, time.Now().Add(10*time.Minute), shop)

	// If shop doesn't exist yet, that's fine - we create it on callback
	// Store nonce in a simpler way as fallback
	c.SetSameSite(http.SameSiteNoneMode)
	c.SetCookie("shopify_nonce", nonce, 600, "/", "", true, true)

	oauthCfg := shopify.OAuthConfig{
		APIKey:      h.cfg.APIKey,
		APISecret:   h.cfg.APISecret,
		Scopes:      h.cfg.Scopes,
		RedirectURI: h.cfg.RedirectURI,
	}

	authURL := shopify.BuildAuthURL(shop, oauthCfg, nonce)
	c.Redirect(http.StatusFound, authURL)
}

// Callback handles the OAuth callback from Shopify.
func (h *Handler) Callback(c *gin.Context) {
	query := c.Request.URL.Query()

	// Validate HMAC
	if !shopify.ValidateCallback(query, h.cfg.APISecret) {
		h.logger.Warn().Msg("invalid OAuth callback HMAC")
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid signature"})
		return
	}

	shop := query.Get("shop")
	if !shopify.ValidateShopDomain(shop) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid shop domain"})
		return
	}

	// Verify state/nonce
	state := query.Get("state")
	nonceCookie, err := c.Cookie("shopify_nonce")
	if err != nil || state != nonceCookie {
		h.logger.Warn().Msg("OAuth state mismatch")
		c.JSON(http.StatusForbidden, gin.H{"error": "state mismatch"})
		return
	}

	// Exchange code for token
	code := query.Get("code")
	oauthCfg := shopify.OAuthConfig{
		APIKey:    h.cfg.APIKey,
		APISecret: h.cfg.APISecret,
	}
	tokenResp, err := shopify.ExchangeToken(c.Request.Context(), shop, code, oauthCfg)
	if err != nil {
		h.logger.Error().Err(err).Msg("token exchange failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authentication failed"})
		return
	}

	ctx := c.Request.Context()

	// Encrypt access token before storage
	encryptedToken, err := encrypt(tokenResp.AccessToken, h.encKey)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to encrypt token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Upsert shop
	var shopID string
	err = h.db.QueryRow(ctx, `
		INSERT INTO shops (shopify_domain, status)
		VALUES ($1, 'active')
		ON CONFLICT (shopify_domain)
		DO UPDATE SET status = 'active', updated_at = NOW()
		RETURNING id
	`, shop).Scan(&shopID)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to upsert shop")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Upsert installation: deactivate old, insert new
	_, _ = h.db.Exec(ctx, `
		UPDATE app_installations SET is_active = FALSE, updated_at = NOW()
		WHERE shop_id = $1 AND is_active = TRUE
	`, shopID)

	_, err = h.db.Exec(ctx, `
		INSERT INTO app_installations (shop_id, access_token, scopes, is_active)
		VALUES ($1, $2, $3, TRUE)
	`, shopID, encryptedToken, tokenResp.Scope)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to save installation")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Create app settings if not exist
	_, err = h.db.Exec(ctx, `
		INSERT INTO app_settings (shop_id) VALUES ($1) ON CONFLICT DO NOTHING
	`, shopID)
	if err != nil {
		h.logger.Warn().Err(err).Msg("failed to create app settings")
	}

	// Create session token for the embedded app
	sessionToken, err := generateNonce()
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to generate session token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	expiresAt := time.Now().Add(time.Duration(h.sessCfg.MaxAge) * time.Second)
	_, err = h.db.Exec(ctx, `
		INSERT INTO shop_sessions (shop_id, session_token, expires_at)
		VALUES ($1, $2, $3)
	`, shopID, sessionToken, expiresAt)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to create session")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Register webhooks
	go h.registerWebhooks(shop, tokenResp.AccessToken)

	// Audit log
	if h.audit != nil {
		h.audit.Log(ctx, shopID, "system", "", "oauth_complete", "shop", shopID, nil, "success", "")
	}

	// Redirect to embedded app
	redirectURL := fmt.Sprintf("https://%s/admin/apps/%s?session=%s", shop, h.cfg.APIKey, sessionToken)
	c.Redirect(http.StatusFound, redirectURL)
}

// registerWebhooks registers all required webhooks with Shopify.
func (h *Handler) registerWebhooks(shop, accessToken string) {
	client := shopify.NewClient(shop, accessToken, h.logger)
	ctx := newBackgroundContext()

	webhooks := map[string]string{
		"APP_UNINSTALLED":       "/webhooks/app/uninstalled",
		"ORDERS_CREATE":         "/webhooks/orders/create",
		"FULFILLMENTS_CREATE":   "/webhooks/fulfillments/create",
		"PRODUCTS_UPDATE":       "/webhooks/products/update",
		"PRODUCTS_DELETE":       "/webhooks/products/delete",
		"INVENTORY_LEVELS_UPDATE": "/webhooks/inventory/update",
		"CUSTOMERS_DATA_REQUEST": "/webhooks/compliance/customers-data-request",
		"CUSTOMERS_REDACT":      "/webhooks/compliance/customers-redact",
		"SHOP_REDACT":           "/webhooks/compliance/shop-redact",
	}

	for topic, path := range webhooks {
		callbackURL := h.cfg.AppURL + path
		if err := client.DeleteAndRegisterWebhook(ctx, topic, callbackURL); err != nil {
			h.logger.Error().Err(err).Str("topic", topic).Msg("failed to register webhook")
		} else {
			h.logger.Info().Str("topic", topic).Msg("webhook registered")
		}
	}
}

func generateNonce() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func newBackgroundContext() context.Context {
	// We intentionally use a background context for async operations
	return context.Background()
}
