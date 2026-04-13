package products

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	Category              string            `json:"category"`
	Status                string            `json:"status"`
	ProcessingDays        int               `json:"processing_days"`
	MarketplaceStockPct   int               `json:"marketplace_stock_percent"`
	ShippingCountries json.RawMessage   `json:"shipping_countries"`
	BlindFulfillment  bool              `json:"blind_fulfillment"`
	Variants           []ListingVariant  `json:"variants,omitempty"`
	SupplierScore      float64           `json:"supplier_score,omitempty"`
	SupplierName       string            `json:"supplier_name,omitempty"`
	AvgResponseHours   float64           `json:"avg_response_hours,omitempty"`
	SupplierCurrency   string            `json:"supplier_currency,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
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
	Category              string               `json:"category"`
	ProcessingDays        int                  `json:"processing_days"`
	MarketplaceStockPct   int                  `json:"marketplace_stock_percent"`
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
	// Validate minimum product info
	if len(input.Title) < 3 {
		return nil, fmt.Errorf("product title must be at least 3 characters")
	}
	if len(input.Description) < 20 {
		return nil, fmt.Errorf("product description must be at least 20 characters. Good descriptions help resellers sell your products")
	}
	if len(input.Variants) == 0 {
		return nil, fmt.Errorf("product must have at least one variant with pricing")
	}

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
	category := input.Category
	if category == "" {
		category = "other"
	}
	stockPct := input.MarketplaceStockPct
	if stockPct <= 0 || stockPct > 100 {
		stockPct = 100
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO supplier_listings (supplier_shop_id, shopify_product_id, title, description, product_type, vendor, tags, images, category, status, processing_days, shipping_countries, blind_fulfillment, marketplace_stock_percent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'draft', $10, $11, $12, $13)
		ON CONFLICT (supplier_shop_id, shopify_product_id) DO UPDATE SET
			title = EXCLUDED.title, description = EXCLUDED.description, product_type = EXCLUDED.product_type,
			vendor = EXCLUDED.vendor, tags = EXCLUDED.tags, images = EXCLUDED.images, category = EXCLUDED.category,
			processing_days = EXCLUDED.processing_days, shipping_countries = EXCLUDED.shipping_countries,
			blind_fulfillment = EXCLUDED.blind_fulfillment, marketplace_stock_percent = EXCLUDED.marketplace_stock_percent
		RETURNING id, supplier_shop_id, shopify_product_id, title, COALESCE(description,''), COALESCE(product_type,''),
			COALESCE(vendor,''), COALESCE(tags,''), images, COALESCE(category,'other'), status, processing_days, COALESCE(marketplace_stock_percent,100), shipping_countries, blind_fulfillment, created_at, updated_at
	`, shopID, input.ShopifyProductID, input.Title, input.Description, input.ProductType,
		input.Vendor, input.Tags, imagesJSON, category, input.ProcessingDays, countriesJSON, input.BlindFulfillment, stockPct,
	).Scan(&listing.ID, &listing.SupplierShopID, &listing.ShopifyProductID, &listing.Title,
		&listing.Description, &listing.ProductType, &listing.Vendor, &listing.Tags, &listing.Images,
		&listing.Category, &listing.Status, &listing.ProcessingDays, &listing.MarketplaceStockPct, &listing.ShippingCountries, &listing.BlindFulfillment,
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
			COALESCE(vendor,''), COALESCE(tags,''), images, COALESCE(category,'other'), status, processing_days, COALESCE(marketplace_stock_percent,100), shipping_countries, blind_fulfillment, created_at, updated_at
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
			&l.ProductType, &l.Vendor, &l.Tags, &l.Images, &l.Category, &l.Status, &l.ProcessingDays, &l.MarketplaceStockPct,
			&l.ShippingCountries, &l.BlindFulfillment, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan listing: %w", err)
		}

		// Load variants with inventory
		vRows, vErr := s.db.Query(ctx, `
			SELECT id, listing_id, shopify_variant_id, COALESCE(title,''), COALESCE(sku,''),
				wholesale_price, COALESCE(suggested_retail_price,0), inventory_quantity,
				COALESCE(weight,0), COALESCE(weight_unit,'kg'), is_active
			FROM supplier_listing_variants WHERE listing_id = $1 ORDER BY created_at
		`, l.ID)
		if vErr == nil {
			for vRows.Next() {
				var v ListingVariant
				vRows.Scan(&v.ID, &v.ListingID, &v.ShopifyVariantID, &v.Title, &v.SKU,
					&v.WholesalePrice, &v.SuggestedRetailPrice, &v.InventoryQuantity,
					&v.Weight, &v.WeightUnit, &v.IsActive)
				l.Variants = append(l.Variants, v)
			}
			vRows.Close()
		}

		listings = append(listings, l)
	}

	return listings, total, nil
}

// UpdateListingStatus changes the status of a listing and cascades to reseller imports.
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

	// Cascade status change to reseller imports
	if status == "paused" || status == "archived" {
		// Block if there are pending/accepted orders for this listing's products
		var pendingCount int
		s.db.QueryRow(ctx, `
			SELECT COUNT(*) FROM routed_orders ro
			JOIN routed_order_items roi ON roi.routed_order_id = ro.id
			JOIN supplier_listing_variants slv ON slv.shopify_variant_id = roi.supplier_variant_id
			WHERE slv.listing_id = $1 AND ro.status IN ('pending', 'accepted', 'processing')
		`, listingID).Scan(&pendingCount)
		if pendingCount > 0 {
			return fmt.Errorf("cannot pause/archive: %d orders are still in progress. Please fulfill or reject them first", pendingCount)
		}

		// Grace period: warn resellers but don't remove immediately
		s.db.Exec(ctx, `
			UPDATE reseller_imports SET last_sync_error = $1
			WHERE supplier_listing_id = $2 AND status = 'active'
		`, "WARNING: Supplier will remove this product soon. Please prepare.", listingID)

		// Notify all resellers who imported this product
		rows, _ := s.db.Query(ctx, `
			SELECT reseller_shop_id FROM reseller_imports
			WHERE supplier_listing_id = $1 AND status = 'active'
		`, listingID)
		if rows != nil {
			var resellerIDs []string
			for rows.Next() {
				var rid string
				rows.Scan(&rid)
				resellerIDs = append(resellerIDs, rid)
			}
			rows.Close()
			for _, rid := range resellerIDs {
				s.db.Exec(ctx, `
					INSERT INTO notifications (shop_id, title, message, type, link)
					VALUES ($1, 'Product Being Removed', $2, 'warning', '/imports')
				`, rid, "A supplier is removing a product you imported. You have 48 hours to prepare. Check your Imports page.")
			}
		}

		s.db.Exec(ctx, `
			UPDATE product_links SET is_active = FALSE
			WHERE supplier_shop_id = $1 AND import_id IN (SELECT id FROM reseller_imports WHERE supplier_listing_id = $2)
		`, shopID, listingID)
	} else if status == "active" {
		// Re-activate imports when listing is published again
		s.db.Exec(ctx, `
			UPDATE reseller_imports SET status = 'active', last_sync_error = NULL
			WHERE supplier_listing_id = $1 AND status = 'paused'
		`, listingID)

		s.db.Exec(ctx, `
			UPDATE product_links SET is_active = TRUE
			WHERE supplier_shop_id = $1 AND import_id IN (SELECT id FROM reseller_imports WHERE supplier_listing_id = $2)
		`, shopID, listingID)
	}

	s.audit.Log(ctx, shopID, "merchant", shopID, "listing_status_changed", "supplier_listing", listingID, map[string]string{"status": status}, "success", "")
	return nil
}

// DeleteListing removes a listing, its variants, and related imports.
func (s *Service) DeleteListing(ctx context.Context, shopID, listingID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Verify ownership
	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM supplier_listings WHERE id = $1 AND supplier_shop_id = $2)`, listingID, shopID).Scan(&exists)
	if err != nil || !exists {
		return fmt.Errorf("listing not found")
	}

	// Block if there are pending/accepted orders
	var pendingCount int
	tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM routed_orders ro
		JOIN routed_order_items roi ON roi.routed_order_id = ro.id
		JOIN supplier_listing_variants slv ON slv.shopify_variant_id = roi.supplier_variant_id
		WHERE slv.listing_id = $1 AND ro.status IN ('pending', 'accepted', 'processing')
	`, listingID).Scan(&pendingCount)
	if pendingCount > 0 {
		return fmt.Errorf("cannot delete: %d orders are still in progress. Please fulfill or reject them first", pendingCount)
	}

	// Mark reseller imports as removed
	tx.Exec(ctx, `UPDATE reseller_imports SET status = 'removed', last_sync_error = 'Supplier deleted this product' WHERE supplier_listing_id = $1 AND status != 'removed'`, listingID)

	// Delete import variants
	tx.Exec(ctx, `DELETE FROM reseller_import_variants WHERE import_id IN (SELECT id FROM reseller_imports WHERE supplier_listing_id = $1)`, listingID)

	// Delete reseller imports
	tx.Exec(ctx, `DELETE FROM reseller_imports WHERE supplier_listing_id = $1`, listingID)

	// Deactivate product links
	tx.Exec(ctx, `UPDATE product_links SET is_active = FALSE WHERE supplier_product_id IN (SELECT shopify_product_id FROM supplier_listings WHERE id = $1) AND supplier_shop_id = $2`, listingID, shopID)

	// Delete listing variants
	tx.Exec(ctx, `DELETE FROM supplier_listing_variants WHERE listing_id = $1`, listingID)

	// Delete the listing
	_, err = tx.Exec(ctx, `DELETE FROM supplier_listings WHERE id = $1 AND supplier_shop_id = $2`, listingID, shopID)
	if err != nil {
		return fmt.Errorf("delete listing: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.audit.Log(ctx, shopID, "merchant", shopID, "listing_deleted", "supplier_listing", listingID, nil, "success", "")
	return nil
}

// GetListing returns a single listing with variants.
func (s *Service) GetListing(ctx context.Context, listingID string) (*SupplierListing, error) {
	var l SupplierListing
	err := s.db.QueryRow(ctx, `
		SELECT id, supplier_shop_id, shopify_product_id, title, COALESCE(description,''), COALESCE(product_type,''),
			COALESCE(vendor,''), COALESCE(tags,''), images, COALESCE(category,'other'), status, processing_days, COALESCE(marketplace_stock_percent,100), shipping_countries, blind_fulfillment, created_at, updated_at
		FROM supplier_listings WHERE id = $1
	`, listingID).Scan(&l.ID, &l.SupplierShopID, &l.ShopifyProductID, &l.Title, &l.Description,
		&l.ProductType, &l.Vendor, &l.Tags, &l.Images, &l.Category, &l.Status, &l.ProcessingDays, &l.MarketplaceStockPct,
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
	baseWhere := `WHERE sl.status = 'active' AND EXISTS (SELECT 1 FROM supplier_listing_variants slv WHERE slv.listing_id = sl.id AND slv.inventory_quantity > 0)`
	args := []interface{}{}
	argN := 1

	if filters.Category != "" {
		baseWhere += fmt.Sprintf(` AND sl.category = $%d`, argN)
		args = append(args, filters.Category)
		argN++
	}
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
	if filters.MaxPrice > 0 {
		baseWhere += fmt.Sprintf(` AND EXISTS (SELECT 1 FROM supplier_listing_variants slvp WHERE slvp.listing_id = sl.id AND slvp.wholesale_price <= $%d)`, argN)
		args = append(args, filters.MaxPrice)
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
			COALESCE(sl.category,'other'), sl.status, sl.processing_days, COALESCE(sl.marketplace_stock_percent,100), sl.shipping_countries, sl.blind_fulfillment, sl.created_at, sl.updated_at,
			COALESCE(sp.reliability_score, 0), COALESCE(sp.company_name, s.name, s.shopify_domain, ''),
			COALESCE(sp.avg_fulfillment_hours, 0), COALESCE(s.currency, 'USD')
		FROM supplier_listings sl
		LEFT JOIN supplier_profiles sp ON sp.shop_id = sl.supplier_shop_id
		LEFT JOIN shops s ON s.id = sl.supplier_shop_id
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
			&l.ProductType, &l.Vendor, &l.Tags, &l.Images, &l.Category, &l.Status, &l.ProcessingDays, &l.MarketplaceStockPct,
			&l.ShippingCountries, &l.BlindFulfillment, &l.CreatedAt, &l.UpdatedAt, &l.SupplierScore, &l.SupplierName, &l.AvgResponseHours, &l.SupplierCurrency); err != nil {
			return nil, 0, fmt.Errorf("scan listing: %w", err)
		}
		listings = append(listings, l)
	}

	// Load variants for each listing
	for i := range listings {
		varRows, err := s.db.Query(ctx, `
			SELECT id, listing_id, shopify_variant_id, COALESCE(title,''), COALESCE(sku,''), wholesale_price,
				COALESCE(suggested_retail_price,0), inventory_quantity, COALESCE(weight,0), COALESCE(weight_unit,'kg'), is_active
			FROM supplier_listing_variants WHERE listing_id = $1 AND is_active = TRUE ORDER BY created_at
		`, listings[i].ID)
		if err != nil {
			continue
		}
		for varRows.Next() {
			var v ListingVariant
			if err := varRows.Scan(&v.ID, &v.ListingID, &v.ShopifyVariantID, &v.Title, &v.SKU,
				&v.WholesalePrice, &v.SuggestedRetailPrice, &v.InventoryQuantity, &v.Weight, &v.WeightUnit, &v.IsActive); err != nil {
				continue
			}
			listings[i].Variants = append(listings[i].Variants, v)
		}
		varRows.Close()
	}

	return listings, total, nil
}

// UpdateListingInput is the input for updating a listing's editable fields.
type UpdateListingInput struct {
	Title               string             `json:"title"`
	Description         string             `json:"description"`
	Category            string             `json:"category"`
	ProcessingDays      int                `json:"processing_days"`
	MarketplaceStockPct int                `json:"marketplace_stock_percent"`
	VariantPrices       map[string]float64 `json:"variant_prices"`
}

// UpdateListing updates a listing's title, description, category, processing_days,
// and optionally variant wholesale prices.
func (s *Service) UpdateListing(ctx context.Context, shopID, listingID string, input UpdateListingInput) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	stockPct := input.MarketplaceStockPct
	if stockPct <= 0 || stockPct > 100 {
		stockPct = 100
	}
	result, err := tx.Exec(ctx, `
		UPDATE supplier_listings
		SET title = $1, description = $2, category = $3, processing_days = $4, marketplace_stock_percent = $5, updated_at = NOW()
		WHERE id = $6 AND supplier_shop_id = $7
	`, input.Title, input.Description, input.Category, input.ProcessingDays, stockPct, listingID, shopID)
	if err != nil {
		return fmt.Errorf("update listing: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("listing not found")
	}

	priceChanged := false
	for variantID, price := range input.VariantPrices {
		_, err := tx.Exec(ctx, `
			UPDATE supplier_listing_variants SET wholesale_price = $1
			WHERE id = $2 AND listing_id = $3
		`, price, variantID, listingID)
		if err != nil {
			return fmt.Errorf("update variant price: %w", err)
		}
		priceChanged = true
	}

	// If prices changed, update all reseller import variant prices
	if priceChanged {
		rows, err := tx.Query(ctx, `
			SELECT riv.id, riv.reseller_price, slv.wholesale_price, ri.markup_type, ri.markup_value
			FROM reseller_import_variants riv
			JOIN supplier_listing_variants slv ON slv.id = riv.supplier_variant_id
			JOIN reseller_imports ri ON ri.id = riv.import_id
			WHERE ri.supplier_listing_id = $1 AND ri.status = 'active'
		`, listingID)
		if err == nil {
			type updateItem struct {
				ID    string
				Price float64
			}
			var updates []updateItem
			for rows.Next() {
				var rivID, markupType string
				var oldPrice, wholesale, markupValue float64
				rows.Scan(&rivID, &oldPrice, &wholesale, &markupType, &markupValue)
				newPrice := calculateResellerPrice(wholesale, markupType, markupValue)
				updates = append(updates, updateItem{ID: rivID, Price: newPrice})
			}
			rows.Close()

			for _, u := range updates {
				tx.Exec(ctx, `UPDATE reseller_import_variants SET reseller_price = $1 WHERE id = $2`, u.Price, u.ID)
			}
			s.logger.Info().Int("reseller_variants_updated", len(updates)).Str("listing_id", listingID).Msg("reseller prices updated from supplier price change")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.audit.Log(ctx, shopID, "merchant", shopID, "listing_updated", "supplier_listing", listingID, map[string]interface{}{
		"title":    input.Title,
		"category": input.Category,
		"price_changed": priceChanged,
	}, "success", "")
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

// MarketplaceFilters holds filtering options for marketplace search.
type MarketplaceFilters struct {
	Search            string  `form:"search"`
	Category          string  `form:"category"`
	ProductType       string  `form:"product_type"`
	Country           string  `form:"country"`
	MaxProcessingDays int     `form:"max_processing_days"`
	MinMargin         float64 `form:"min_margin"`
	MaxPrice          float64 `form:"max_price"`
}
