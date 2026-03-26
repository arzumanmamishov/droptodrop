package imports

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/droptodrop/droptodrop/internal/audit"
	"github.com/droptodrop/droptodrop/internal/queue"
)

// Import represents a reseller's imported product.
type Import struct {
	ID                string          `json:"id"`
	ResellerShopID    string          `json:"reseller_shop_id"`
	SupplierListingID string          `json:"supplier_listing_id"`
	ShopifyProductID  *int64          `json:"shopify_product_id,omitempty"`
	Status            string          `json:"status"`
	MarkupType        string          `json:"markup_type"`
	MarkupValue       float64         `json:"markup_value"`
	SyncImages        bool            `json:"sync_images"`
	SyncDescription   bool            `json:"sync_description"`
	SyncTitle         bool            `json:"sync_title"`
	LastSyncAt        *time.Time      `json:"last_sync_at,omitempty"`
	LastSyncError     *string         `json:"last_sync_error,omitempty"`
	Variants          []ImportVariant `json:"variants,omitempty"`
	SupplierTitle     string          `json:"supplier_title,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// ImportVariant represents a variant in the reseller's imported product.
type ImportVariant struct {
	ID                string  `json:"id"`
	ImportID          string  `json:"import_id"`
	SupplierVariantID string  `json:"supplier_variant_id"`
	ShopifyVariantID  *int64  `json:"shopify_variant_id,omitempty"`
	ResellerPrice     float64 `json:"reseller_price"`
}

// ImportInput is the input for importing a supplier listing.
type ImportInput struct {
	SupplierListingID string  `json:"supplier_listing_id" binding:"required"`
	MarkupType        string  `json:"markup_type"`
	MarkupValue       float64 `json:"markup_value"`
	SyncImages        bool    `json:"sync_images"`
	SyncDescription   bool    `json:"sync_description"`
	SyncTitle         bool    `json:"sync_title"`
}

// Service handles import operations.
type Service struct {
	db     *pgxpool.Pool
	queue  *queue.Client
	logger zerolog.Logger
	audit  *audit.Service
}

// NewService creates an imports service.
func NewService(db *pgxpool.Pool, q *queue.Client, logger zerolog.Logger, auditSvc *audit.Service) *Service {
	return &Service{db: db, queue: q, logger: logger, audit: auditSvc}
}

// Create initiates a product import from a supplier listing to a reseller store.
func (s *Service) Create(ctx context.Context, resellerShopID string, input ImportInput) (*Import, error) {
	// Verify the listing exists and is active
	var listingStatus string
	err := s.db.QueryRow(ctx, `SELECT status FROM supplier_listings WHERE id = $1`, input.SupplierListingID).Scan(&listingStatus)
	if err != nil {
		return nil, fmt.Errorf("listing not found: %w", err)
	}
	if listingStatus != "active" {
		return nil, fmt.Errorf("listing is not active")
	}

	markupType := input.MarkupType
	if markupType == "" {
		markupType = "percentage"
	}
	markupValue := input.MarkupValue
	if markupValue == 0 {
		markupValue = 30
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var imp Import
	err = tx.QueryRow(ctx, `
		INSERT INTO reseller_imports (reseller_shop_id, supplier_listing_id, status, markup_type, markup_value, sync_images, sync_description, sync_title)
		VALUES ($1, $2, 'pending', $3, $4, $5, $6, $7)
		ON CONFLICT (reseller_shop_id, supplier_listing_id) DO UPDATE SET
			status = 'pending', markup_type = EXCLUDED.markup_type, markup_value = EXCLUDED.markup_value,
			sync_images = EXCLUDED.sync_images, sync_description = EXCLUDED.sync_description, sync_title = EXCLUDED.sync_title
		RETURNING id, reseller_shop_id, supplier_listing_id, shopify_product_id, status, markup_type, markup_value,
			sync_images, sync_description, sync_title, last_sync_at, last_sync_error, created_at, updated_at
	`, resellerShopID, input.SupplierListingID, markupType, markupValue,
		input.SyncImages, input.SyncDescription, input.SyncTitle,
	).Scan(&imp.ID, &imp.ResellerShopID, &imp.SupplierListingID, &imp.ShopifyProductID,
		&imp.Status, &imp.MarkupType, &imp.MarkupValue, &imp.SyncImages, &imp.SyncDescription,
		&imp.SyncTitle, &imp.LastSyncAt, &imp.LastSyncError, &imp.CreatedAt, &imp.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create import: %w", err)
	}

	// Create import variants for each listing variant
	rows, err := tx.Query(ctx, `
		SELECT id, wholesale_price, COALESCE(suggested_retail_price, 0)
		FROM supplier_listing_variants WHERE listing_id = $1 AND is_active = TRUE
	`, input.SupplierListingID)
	if err != nil {
		return nil, fmt.Errorf("get variants: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var supplierVariantID string
		var wholesalePrice, suggestedRetail float64
		if err := rows.Scan(&supplierVariantID, &wholesalePrice, &suggestedRetail); err != nil {
			return nil, fmt.Errorf("scan variant: %w", err)
		}

		resellerPrice := calculateResellerPrice(wholesalePrice, markupType, markupValue)

		var iv ImportVariant
		err = tx.QueryRow(ctx, `
			INSERT INTO reseller_import_variants (import_id, supplier_variant_id, reseller_price)
			VALUES ($1, $2, $3)
			ON CONFLICT (import_id, supplier_variant_id) DO UPDATE SET reseller_price = EXCLUDED.reseller_price
			RETURNING id, import_id, supplier_variant_id, shopify_variant_id, reseller_price
		`, imp.ID, supplierVariantID, resellerPrice,
		).Scan(&iv.ID, &iv.ImportID, &iv.SupplierVariantID, &iv.ShopifyVariantID, &iv.ResellerPrice)
		if err != nil {
			return nil, fmt.Errorf("create import variant: %w", err)
		}
		imp.Variants = append(imp.Variants, iv)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// Enqueue product creation job
	_, err = s.queue.Enqueue(ctx, "imports", "create_product", map[string]string{
		"import_id":       imp.ID,
		"reseller_shop_id": resellerShopID,
	}, 3)
	if err != nil {
		s.logger.Error().Err(err).Str("import_id", imp.ID).Msg("failed to enqueue import job")
	}

	s.audit.Log(ctx, resellerShopID, "merchant", resellerShopID, "product_imported", "reseller_import", imp.ID,
		map[string]string{"listing_id": input.SupplierListingID}, "success", "")

	return &imp, nil
}

// List returns imports for a reseller shop.
func (s *Service) List(ctx context.Context, resellerShopID string, limit, offset int) ([]Import, int, error) {
	var total int
	err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM reseller_imports WHERE reseller_shop_id = $1`, resellerShopID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count imports: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT ri.id, ri.reseller_shop_id, ri.supplier_listing_id, ri.shopify_product_id, ri.status,
			ri.markup_type, ri.markup_value, ri.sync_images, ri.sync_description, ri.sync_title,
			ri.last_sync_at, ri.last_sync_error, ri.created_at, ri.updated_at,
			COALESCE(sl.title, '') as supplier_title
		FROM reseller_imports ri
		LEFT JOIN supplier_listings sl ON sl.id = ri.supplier_listing_id
		WHERE ri.reseller_shop_id = $1
		ORDER BY ri.updated_at DESC
		LIMIT $2 OFFSET $3
	`, resellerShopID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list imports: %w", err)
	}
	defer rows.Close()

	var imports []Import
	for rows.Next() {
		var imp Import
		if err := rows.Scan(&imp.ID, &imp.ResellerShopID, &imp.SupplierListingID, &imp.ShopifyProductID,
			&imp.Status, &imp.MarkupType, &imp.MarkupValue, &imp.SyncImages, &imp.SyncDescription,
			&imp.SyncTitle, &imp.LastSyncAt, &imp.LastSyncError, &imp.CreatedAt, &imp.UpdatedAt,
			&imp.SupplierTitle); err != nil {
			return nil, 0, fmt.Errorf("scan import: %w", err)
		}
		imports = append(imports, imp)
	}

	return imports, total, nil
}

// ResyncImport triggers a manual re-sync of an imported product.
func (s *Service) ResyncImport(ctx context.Context, resellerShopID, importID string) error {
	var status string
	err := s.db.QueryRow(ctx, `SELECT status FROM reseller_imports WHERE id = $1 AND reseller_shop_id = $2`, importID, resellerShopID).Scan(&status)
	if err != nil {
		return fmt.Errorf("import not found: %w", err)
	}

	_, err = s.queue.Enqueue(ctx, "imports", "sync_product", map[string]string{
		"import_id":       importID,
		"reseller_shop_id": resellerShopID,
	}, 3)
	if err != nil {
		return fmt.Errorf("enqueue sync job: %w", err)
	}

	s.audit.Log(ctx, resellerShopID, "merchant", resellerShopID, "import_resync_requested", "reseller_import", importID, nil, "success", "")
	return nil
}

func calculateResellerPrice(wholesale float64, markupType string, markupValue float64) float64 {
	switch markupType {
	case "fixed":
		return wholesale + markupValue
	case "percentage":
		return wholesale * (1 + markupValue/100)
	default:
		return wholesale * 1.3
	}
}
