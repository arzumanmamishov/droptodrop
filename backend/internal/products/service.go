package products

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/droptodrop/droptodrop/internal/audit"
)

// SupplierListing represents a product listed by a supplier.
type SupplierListing struct {
	ID                string            `json:"id"`
	SupplierShopID    string            `json:"supplier_shop_id"`
	ShopifyProductID  int64             `json:"shopify_product_id"`
	Title             string            `json:"title"`
	Description       string            `json:"description"`
	ProductType       string            `json:"product_type"`
	Vendor            string            `json:"vendor"`
	Tags              string            `json:"tags"`
	Images            json.RawMessage   `json:"images"`
	Status            string            `json:"status"`
	ProcessingDays    int               `json:"processing_days"`
	ShippingCountries json.RawMessage   `json:"shipping_countries"`
	BlindFulfillment  bool              `json:"blind_fulfillment"`
	Variants          []ListingVariant  `json:"variants,omitempty"`
	CreatedAt         string            `json:"created_at"`
	UpdatedAt         string            `json:"updated_at"`
}

// ListingVariant represents a variant within a supplier listing.
type ListingVariant struct {
	ID                   string  `json:"id"`
	ListingID            string  `json:"listing_id"`
	ShopifyVariantID     int64   `json:"shopify_variant_id"`
	Title                string  `json:"title"`
	SKU                  string  `json:"sku"`
	WholesalePrice       float64 `json:"wholesale_price"`
	SuggestedRetailPrice float64 `json:"suggested_retail_price"`
	InventoryQuantity    int     `json:"inventory_quantity"`
	Weight               float64 `json:"weight"`
	WeightUnit           string  `json:"weight_unit"`
	IsActive             bool    `json:"is_active"`
}

// CreateListingInput is the input for creating a listing.
type CreateListingInput struct {
	ShopifyProductID  int64                `json:"shopify_product_id"`
	Title             string               `json:"title"`
	Description       string               `json:"description"`
	ProductType       string               `json:"product_type"`
	Vendor            string               `json:"vendor"`
	Tags              string               `json:"tags"`
	Images            json.RawMessage      `json:"images"`
	ProcessingDays    int                  `json:"processing_days"`
	ShippingCountries []string             `json:"shipping_countries"`
	BlindFulfillment  bool                 `json:"blind_fulfillment"`
	Variants          []CreateVariantInput `json:"variants"`
}

// CreateVariantInput is the input for creating a variant.
type CreateVariantInput struct {
	ShopifyVariantID     int64   `json:"shopify_variant_id"`
	Title                string  `json:"title"`
	SKU                  string  `json:"sku"`
	WholesalePrice       float64 `json:"wholesale_price"`
	SuggestedRetailPrice float64 `json:"suggested_retail_price"`
	InventoryQuantity    int     `json:"inventory_quantity"`
	Weight               float64 `json:"weight"`
	WeightUnit           string  `json:"weight_unit"`
}

// Service handles product listing operations.
type Service struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
	audit  *audit.Service
}

// NewService creates a product service.
func NewService(db *pgxpool.Pool, logger zerolog.Logger, auditSvc *audit.Service) *Service {
	return &Service{db: db, logger: logger, audit: auditSvc}
}

// CreateListing creates a new supplier listing with variants.
func (s *Service) CreateListing(ctx context.Context, shopID string, input CreateListingInput) (*SupplierListing, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	countriesJSON, _ := json.Marshal(input.ShippingCountries)
	imagesJSON := input.Images
	if imagesJSON == nil {
		imagesJSON = json.RawMessage("[]")
	}

	var listing SupplierListing
	err = tx.QueryRow(ctx, `
		INSERT INTO supplier_listings (supplier_shop_id, shopify_product_id, title, description, product_type, vendor, tags, images, status, processing_days, shipping_countries, blind_fulfillment)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'draft', $9, $10, $11)
		ON CONFLICT (supplier_shop_id, shopify_product_id) DO UPDATE SET
			title = EXCLUDED.title, description = EXCLUDED.description, product_type = EXCLUDED.product_type,
			vendor = EXCLUDED.vendor, tags = EXCLUDED.tags, images = EXCLUDED.images,
			processing_days = EXCLUDED.processing_days, shipping_countries = EXCLUDED.shipping_countries,
			blind_fulfillment = EXCLUDED.blind_fulfillment
		RETURNING id, supplier_shop_id, shopify_product_id, title, COALESCE(description,''), COALESCE(product_type,''),
			COALESCE(vendor,''), COALESCE(tags,''), images, status, processing_days, shipping_countries, blind_fulfillment, created_at, updated_at
	`, shopID, input.ShopifyProductID, input.Title, input.Description, input.ProductType,
		input.Vendor, input.Tags, imagesJSON, input.ProcessingDays, countriesJSON, input.BlindFulfillment,
	).Scan(&listing.ID, &listing.SupplierShopID, &listing.ShopifyProductID, &listing.Title,
		&listing.Description, &listing.ProductType, &listing.Vendor, &listing.Tags, &listing.Images,
		&listing.Status, &listing.ProcessingDays, &listing.ShippingCountries, &listing.BlindFulfillment,
		&listing.CreatedAt, &listing.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert listing: %w", err)
	}

	// Insert variants
	for _, v := range input.Variants {
		var variant ListingVariant
		err = tx.QueryRow(ctx, `
			INSERT INTO supplier_listing_variants (listing_id, shopify_variant_id, title, sku, wholesale_price, suggested_retail_price, inventory_quantity, weight, weight_unit)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (listing_id, shopify_variant_id) DO UPDATE SET
				title = EXCLUDED.title, sku = EXCLUDED.sku, wholesale_price = EXCLUDED.wholesale_price,
				suggested_retail_price = EXCLUDED.suggested_retail_price, inventory_quantity = EXCLUDED.inventory_quantity,
				weight = EXCLUDED.weight, weight_unit = EXCLUDED.weight_unit
			RETURNING id, listing_id, shopify_variant_id, COALESCE(title,''), COALESCE(sku,''), wholesale_price, COALESCE(suggested_retail_price,0), inventory_quantity, COALESCE(weight,0), COALESCE(weight_unit,'kg'), is_active
		`, listing.ID, v.ShopifyVariantID, v.Title, v.SKU, v.WholesalePrice, v.SuggestedRetailPrice,
			v.InventoryQuantity, v.Weight, v.WeightUnit,
		).Scan(&variant.ID, &variant.ListingID, &variant.ShopifyVariantID, &variant.Title,
			&variant.SKU, &variant.WholesalePrice, &variant.SuggestedRetailPrice,
			&variant.InventoryQuantity, &variant.Weight, &variant.WeightUnit, &variant.IsActive)
		if err != nil {
			return nil, fmt.Errorf("insert variant: %w", err)
		}
		listing.Variants = append(listing.Variants, variant)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	s.audit.Log(ctx, shopID, "merchant", shopID, "listing_created", "supplier_listing", listing.ID, map[string]interface{}{"product_id": input.ShopifyProductID}, "success", "")
	return &listing, nil
}

// ListSupplierListings returns listings for a supplier shop.
func (s *Service) ListSupplierListings(ctx context.Context, shopID string, status string, limit, offset int) ([]SupplierListing, int, error) {
	countQuery := `SELECT COUNT(*) FROM supplier_listings WHERE supplier_shop_id = $1`
	listQuery := `
		SELECT id, supplier_shop_id, shopify_product_id, title, COALESCE(description,''), COALESCE(product_type,''),
			COALESCE(vendor,''), COALESCE(tags,''), images, status, processing_days, shipping_countries, blind_fulfillment, created_at, updated_at
		FROM supplier_listings WHERE supplier_shop_id = $1`

	args := []interface{}{shopID}
	if status != "" {
		countQuery += ` AND status = $2`
		listQuery += ` AND status = $2`
		args = append(args, status)
	}

	var total int
	err := s.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count listings: %w", err)
	}

	listQuery += fmt.Sprintf(` ORDER BY updated_at DESC LIMIT %d OFFSET %d`, limit, offset)
	rows, err := s.db.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list listings: %w", err)
	}
	defer rows.Close()

	var listings []SupplierListing
	for rows.Next() {
		var l SupplierListing
		if err := rows.Scan(&l.ID, &l.SupplierShopID, &l.ShopifyProductID, &l.Title, &l.Description,
			&l.ProductType, &l.Vendor, &l.Tags, &l.Images, &l.Status, &l.ProcessingDays,
			&l.ShippingCountries, &l.BlindFulfillment, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan listing: %w", err)
		}
		listings = append(listings, l)
	}

	return listings, total, nil
}

// UpdateListingStatus changes the status of a listing.
func (s *Service) UpdateListingStatus(ctx context.Context, shopID, listingID, status string) error {
	result, err := s.db.Exec(ctx, `
		UPDATE supplier_listings SET status = $1 WHERE id = $2 AND supplier_shop_id = $3
	`, status, listingID, shopID)
	if err != nil {
		return fmt.Errorf("update listing status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("listing not found")
	}

	s.audit.Log(ctx, shopID, "merchant", shopID, "listing_status_changed", "supplier_listing", listingID, map[string]string{"status": status}, "success", "")
	return nil
}

// GetListing returns a single listing with variants.
func (s *Service) GetListing(ctx context.Context, listingID string) (*SupplierListing, error) {
	var l SupplierListing
	err := s.db.QueryRow(ctx, `
		SELECT id, supplier_shop_id, shopify_product_id, title, COALESCE(description,''), COALESCE(product_type,''),
			COALESCE(vendor,''), COALESCE(tags,''), images, status, processing_days, shipping_countries, blind_fulfillment, created_at, updated_at
		FROM supplier_listings WHERE id = $1
	`, listingID).Scan(&l.ID, &l.SupplierShopID, &l.ShopifyProductID, &l.Title, &l.Description,
		&l.ProductType, &l.Vendor, &l.Tags, &l.Images, &l.Status, &l.ProcessingDays,
		&l.ShippingCountries, &l.BlindFulfillment, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get listing: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, listing_id, shopify_variant_id, COALESCE(title,''), COALESCE(sku,''), wholesale_price,
			COALESCE(suggested_retail_price,0), inventory_quantity, COALESCE(weight,0), COALESCE(weight_unit,'kg'), is_active
		FROM supplier_listing_variants WHERE listing_id = $1 ORDER BY created_at
	`, listingID)
	if err != nil {
		return nil, fmt.Errorf("get variants: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var v ListingVariant
		if err := rows.Scan(&v.ID, &v.ListingID, &v.ShopifyVariantID, &v.Title, &v.SKU,
			&v.WholesalePrice, &v.SuggestedRetailPrice, &v.InventoryQuantity,
			&v.Weight, &v.WeightUnit, &v.IsActive); err != nil {
			return nil, fmt.Errorf("scan variant: %w", err)
		}
		l.Variants = append(l.Variants, v)
	}

	return &l, nil
}

// ListMarketplace returns active listings from all suppliers for the marketplace view.
func (s *Service) ListMarketplace(ctx context.Context, filters MarketplaceFilters, limit, offset int) ([]SupplierListing, int, error) {
	baseWhere := `WHERE sl.status = 'active'`
	args := []interface{}{}
	argN := 1

	if filters.ProductType != "" {
		baseWhere += fmt.Sprintf(` AND sl.product_type = $%d`, argN)
		args = append(args, filters.ProductType)
		argN++
	}
	if filters.Search != "" {
		baseWhere += fmt.Sprintf(` AND (sl.title ILIKE $%d OR sl.description ILIKE $%d)`, argN, argN)
		args = append(args, "%"+filters.Search+"%")
		argN++
	}
	if filters.MaxProcessingDays > 0 {
		baseWhere += fmt.Sprintf(` AND sl.processing_days <= $%d`, argN)
		args = append(args, filters.MaxProcessingDays)
		argN++
	}

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM supplier_listings sl %s`, baseWhere)
	err := s.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count marketplace: %w", err)
	}

	listQuery := fmt.Sprintf(`
		SELECT sl.id, sl.supplier_shop_id, sl.shopify_product_id, sl.title, COALESCE(sl.description,''),
			COALESCE(sl.product_type,''), COALESCE(sl.vendor,''), COALESCE(sl.tags,''), sl.images,
			sl.status, sl.processing_days, sl.shipping_countries, sl.blind_fulfillment, sl.created_at, sl.updated_at
		FROM supplier_listings sl
		%s ORDER BY sl.updated_at DESC LIMIT %d OFFSET %d
	`, baseWhere, limit, offset)

	rows, err := s.db.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list marketplace: %w", err)
	}
	defer rows.Close()

	var listings []SupplierListing
	for rows.Next() {
		var l SupplierListing
		if err := rows.Scan(&l.ID, &l.SupplierShopID, &l.ShopifyProductID, &l.Title, &l.Description,
			&l.ProductType, &l.Vendor, &l.Tags, &l.Images, &l.Status, &l.ProcessingDays,
			&l.ShippingCountries, &l.BlindFulfillment, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan listing: %w", err)
		}
		listings = append(listings, l)
	}

	return listings, total, nil
}

// MarketplaceFilters holds filtering options for marketplace search.
type MarketplaceFilters struct {
	Search            string `form:"search"`
	ProductType       string `form:"product_type"`
	Country           string `form:"country"`
	MaxProcessingDays int    `form:"max_processing_days"`
	MinMargin         float64 `form:"min_margin"`
	MaxPrice          float64 `form:"max_price"`
}
