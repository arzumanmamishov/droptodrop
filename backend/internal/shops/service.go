package shops

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/droptodrop/droptodrop/internal/audit"
)

// Shop represents a merchant shop.
type Shop struct {
	ID            string `json:"id"`
	ShopifyDomain string `json:"shopify_domain"`
	ShopifyShopID *int64 `json:"shopify_shop_id,omitempty"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	Currency      string `json:"currency"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// SupplierProfile represents supplier-specific settings.
type SupplierProfile struct {
	ID                   string   `json:"id"`
	ShopID               string   `json:"shop_id"`
	IsEnabled            bool     `json:"is_enabled"`
	DefaultProcessingDays int     `json:"default_processing_days"`
	ShippingCountries    []string `json:"shipping_countries"`
	BlindFulfillment     bool     `json:"blind_fulfillment"`
	ResellerApprovalMode string   `json:"reseller_approval_mode"`
	CompanyName          string   `json:"company_name"`
	SupportEmail         string   `json:"support_email"`
	ReturnPolicyURL      string   `json:"return_policy_url"`
}

// ResellerProfile represents reseller-specific settings.
type ResellerProfile struct {
	ID                 string  `json:"id"`
	ShopID             string  `json:"shop_id"`
	IsEnabled          bool    `json:"is_enabled"`
	DefaultMarkupType  string  `json:"default_markup_type"`
	DefaultMarkupValue float64 `json:"default_markup_value"`
	MinMarginPercentage float64 `json:"min_margin_percentage"`
	AutoSyncInventory  bool    `json:"auto_sync_inventory"`
	AutoSyncPrice      bool    `json:"auto_sync_price"`
	AutoSyncContent    bool    `json:"auto_sync_content"`
}

// Service handles shop operations.
type Service struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
	audit  *audit.Service
}

// NewService creates a shop service.
func NewService(db *pgxpool.Pool, logger zerolog.Logger, auditSvc *audit.Service) *Service {
	return &Service{db: db, logger: logger, audit: auditSvc}
}

// GetByID returns a shop by ID.
func (s *Service) GetByID(ctx context.Context, id string) (*Shop, error) {
	var shop Shop
	err := s.db.QueryRow(ctx, `
		SELECT id, shopify_domain, shopify_shop_id, COALESCE(name,''), COALESCE(email,''), role, status, currency, created_at, updated_at
		FROM shops WHERE id = $1
	`, id).Scan(&shop.ID, &shop.ShopifyDomain, &shop.ShopifyShopID, &shop.Name, &shop.Email, &shop.Role, &shop.Status, &shop.Currency, &shop.CreatedAt, &shop.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get shop: %w", err)
	}
	return &shop, nil
}

// SetRole sets the role for a shop and creates the corresponding profile.
func (s *Service) SetRole(ctx context.Context, shopID, role string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `UPDATE shops SET role = $1 WHERE id = $2`, role, shopID)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}

	switch role {
	case "supplier":
		_, err = tx.Exec(ctx, `
			INSERT INTO supplier_profiles (shop_id) VALUES ($1) ON CONFLICT DO NOTHING
		`, shopID)
	case "reseller":
		_, err = tx.Exec(ctx, `
			INSERT INTO reseller_profiles (shop_id) VALUES ($1) ON CONFLICT DO NOTHING
		`, shopID)
	default:
		return fmt.Errorf("invalid role: %s", role)
	}
	if err != nil {
		return fmt.Errorf("create profile: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.audit.Log(ctx, shopID, "merchant", shopID, "role_set", "shop", shopID, map[string]string{"role": role}, "success", "")
	return nil
}

// GetSupplierProfile returns the supplier profile for a shop.
func (s *Service) GetSupplierProfile(ctx context.Context, shopID string) (*SupplierProfile, error) {
	var p SupplierProfile
	err := s.db.QueryRow(ctx, `
		SELECT id, shop_id, is_enabled, default_processing_days, blind_fulfillment, reseller_approval_mode,
			COALESCE(company_name,''), COALESCE(support_email,''), COALESCE(return_policy_url,'')
		FROM supplier_profiles WHERE shop_id = $1
	`, shopID).Scan(&p.ID, &p.ShopID, &p.IsEnabled, &p.DefaultProcessingDays, &p.BlindFulfillment,
		&p.ResellerApprovalMode, &p.CompanyName, &p.SupportEmail, &p.ReturnPolicyURL)
	if err != nil {
		return nil, fmt.Errorf("get supplier profile: %w", err)
	}
	return &p, nil
}

// UpdateSupplierProfile updates supplier settings.
func (s *Service) UpdateSupplierProfile(ctx context.Context, shopID string, update map[string]interface{}) error {
	_, err := s.db.Exec(ctx, `
		UPDATE supplier_profiles SET
			is_enabled = COALESCE($2, is_enabled),
			default_processing_days = COALESCE($3, default_processing_days),
			blind_fulfillment = COALESCE($4, blind_fulfillment),
			reseller_approval_mode = COALESCE($5, reseller_approval_mode),
			company_name = COALESCE($6, company_name),
			support_email = COALESCE($7, support_email),
			return_policy_url = COALESCE($8, return_policy_url)
		WHERE shop_id = $1
	`, shopID,
		update["is_enabled"],
		update["default_processing_days"],
		update["blind_fulfillment"],
		update["reseller_approval_mode"],
		update["company_name"],
		update["support_email"],
		update["return_policy_url"],
	)
	if err != nil {
		return fmt.Errorf("update supplier profile: %w", err)
	}

	s.audit.Log(ctx, shopID, "merchant", shopID, "supplier_profile_updated", "supplier_profile", shopID, update, "success", "")
	return nil
}

// GetResellerProfile returns the reseller profile for a shop.
func (s *Service) GetResellerProfile(ctx context.Context, shopID string) (*ResellerProfile, error) {
	var p ResellerProfile
	err := s.db.QueryRow(ctx, `
		SELECT id, shop_id, is_enabled, default_markup_type, default_markup_value, min_margin_percentage,
			auto_sync_inventory, auto_sync_price, auto_sync_content
		FROM reseller_profiles WHERE shop_id = $1
	`, shopID).Scan(&p.ID, &p.ShopID, &p.IsEnabled, &p.DefaultMarkupType, &p.DefaultMarkupValue,
		&p.MinMarginPercentage, &p.AutoSyncInventory, &p.AutoSyncPrice, &p.AutoSyncContent)
	if err != nil {
		return nil, fmt.Errorf("get reseller profile: %w", err)
	}
	return &p, nil
}

// Deactivate soft-deactivates a shop on uninstall.
func (s *Service) Deactivate(ctx context.Context, shopDomain string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var shopID string
	err = tx.QueryRow(ctx, `UPDATE shops SET status = 'uninstalled' WHERE shopify_domain = $1 RETURNING id`, shopDomain).Scan(&shopID)
	if err != nil {
		return fmt.Errorf("deactivate shop: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE app_installations SET is_active = FALSE, uninstalled_at = NOW() WHERE shop_id = $1 AND is_active = TRUE
	`, shopID)
	if err != nil {
		return fmt.Errorf("deactivate installation: %w", err)
	}

	// Expire all sessions
	_, err = tx.Exec(ctx, `DELETE FROM shop_sessions WHERE shop_id = $1`, shopID)
	if err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.audit.Log(ctx, shopID, "system", "", "app_uninstalled", "shop", shopID, nil, "success", "")
	return nil
}
