package advanced

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type Service struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
}

func NewService(db *pgxpool.Pool, logger zerolog.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// ===== REVIEWS =====

type Review struct {
	ID              string    `json:"id"`
	SupplierShopID  string    `json:"supplier_shop_id"`
	ResellerShopID  string    `json:"reseller_shop_id"`
	RoutedOrderID   *string   `json:"routed_order_id,omitempty"`
	Rating          int       `json:"rating"`
	Title           string    `json:"title"`
	Comment         string    `json:"comment"`
	CreatedAt       time.Time `json:"created_at"`
	ResellerDomain  string    `json:"reseller_domain,omitempty"`
}

type ReviewSummary struct {
	AverageRating float64 `json:"average_rating"`
	TotalReviews  int     `json:"total_reviews"`
	FiveStar      int     `json:"five_star"`
	FourStar      int     `json:"four_star"`
	ThreeStar     int     `json:"three_star"`
	TwoStar       int     `json:"two_star"`
	OneStar       int     `json:"one_star"`
}

func (s *Service) CreateReview(ctx context.Context, supplierShopID, resellerShopID string, orderID *string, rating int, title, comment string) (*Review, error) {
	if rating < 1 || rating > 5 {
		return nil, fmt.Errorf("rating must be 1-5")
	}
	var r Review
	err := s.db.QueryRow(ctx, `
		INSERT INTO reviews (supplier_shop_id, reseller_shop_id, routed_order_id, rating, title, comment)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, supplier_shop_id, reseller_shop_id, routed_order_id, rating, COALESCE(title,''), COALESCE(comment,''), created_at
	`, supplierShopID, resellerShopID, orderID, rating, title, comment).Scan(
		&r.ID, &r.SupplierShopID, &r.ResellerShopID, &r.RoutedOrderID, &r.Rating, &r.Title, &r.Comment, &r.CreatedAt)
	return &r, err
}

func (s *Service) GetSupplierReviews(ctx context.Context, supplierShopID string) ([]Review, *ReviewSummary, error) {
	rows, err := s.db.Query(ctx, `
		SELECT r.id, r.supplier_shop_id, r.reseller_shop_id, r.routed_order_id, r.rating,
			COALESCE(r.title,''), COALESCE(r.comment,''), r.created_at, COALESCE(s.shopify_domain,'')
		FROM reviews r LEFT JOIN shops s ON s.id = r.reseller_shop_id
		WHERE r.supplier_shop_id = $1 AND r.is_public = TRUE
		ORDER BY r.created_at DESC LIMIT 50
	`, supplierShopID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var reviews []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.SupplierShopID, &r.ResellerShopID, &r.RoutedOrderID, &r.Rating, &r.Title, &r.Comment, &r.CreatedAt, &r.ResellerDomain); err != nil {
			return nil, nil, err
		}
		reviews = append(reviews, r)
	}

	summary := &ReviewSummary{}
	s.db.QueryRow(ctx, `
		SELECT COALESCE(AVG(rating),0), COUNT(*),
			COUNT(*) FILTER (WHERE rating=5), COUNT(*) FILTER (WHERE rating=4),
			COUNT(*) FILTER (WHERE rating=3), COUNT(*) FILTER (WHERE rating=2), COUNT(*) FILTER (WHERE rating=1)
		FROM reviews WHERE supplier_shop_id = $1 AND is_public = TRUE
	`, supplierShopID).Scan(&summary.AverageRating, &summary.TotalReviews,
		&summary.FiveStar, &summary.FourStar, &summary.ThreeStar, &summary.TwoStar, &summary.OneStar)

	return reviews, summary, nil
}

// ===== SHIPPING RULES =====

type ShippingRule struct {
	ID                    string   `json:"id"`
	SupplierShopID        string   `json:"supplier_shop_id"`
	CountryCode           string   `json:"country_code"`
	ShippingRate          float64  `json:"shipping_rate"`
	FreeShippingThreshold *float64 `json:"free_shipping_threshold,omitempty"`
	EstDaysMin            int      `json:"estimated_days_min"`
	EstDaysMax            int      `json:"estimated_days_max"`
	IsActive              bool     `json:"is_active"`
}

func (s *Service) UpsertShippingRule(ctx context.Context, supplierShopID string, rule ShippingRule) (*ShippingRule, error) {
	var r ShippingRule
	err := s.db.QueryRow(ctx, `
		INSERT INTO shipping_rules (supplier_shop_id, country_code, shipping_rate, free_shipping_threshold, estimated_days_min, estimated_days_max)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (supplier_shop_id, country_code) DO UPDATE SET
			shipping_rate = $3, free_shipping_threshold = $4, estimated_days_min = $5, estimated_days_max = $6
		RETURNING id, supplier_shop_id, country_code, shipping_rate, free_shipping_threshold, estimated_days_min, estimated_days_max, is_active
	`, supplierShopID, rule.CountryCode, rule.ShippingRate, rule.FreeShippingThreshold, rule.EstDaysMin, rule.EstDaysMax).Scan(
		&r.ID, &r.SupplierShopID, &r.CountryCode, &r.ShippingRate, &r.FreeShippingThreshold, &r.EstDaysMin, &r.EstDaysMax, &r.IsActive)
	return &r, err
}

func (s *Service) ListShippingRules(ctx context.Context, supplierShopID string) ([]ShippingRule, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, supplier_shop_id, country_code, shipping_rate, free_shipping_threshold, estimated_days_min, estimated_days_max, is_active
		FROM shipping_rules WHERE supplier_shop_id = $1 ORDER BY country_code
	`, supplierShopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []ShippingRule
	for rows.Next() {
		var r ShippingRule
		if err := rows.Scan(&r.ID, &r.SupplierShopID, &r.CountryCode, &r.ShippingRate, &r.FreeShippingThreshold, &r.EstDaysMin, &r.EstDaysMax, &r.IsActive); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// ===== SAMPLE ORDERS =====

type SampleOrder struct {
	ID                string    `json:"id"`
	ResellerShopID    string    `json:"reseller_shop_id"`
	SupplierListingID string    `json:"supplier_listing_id"`
	Status            string    `json:"status"`
	Quantity          int       `json:"quantity"`
	Notes             string    `json:"notes"`
	TrackingNumber    string    `json:"tracking_number"`
	CreatedAt         time.Time `json:"created_at"`
	ListingTitle      string    `json:"listing_title,omitempty"`
}

func (s *Service) CreateSampleOrder(ctx context.Context, resellerShopID, listingID string, quantity int, notes string) (*SampleOrder, error) {
	var so SampleOrder
	err := s.db.QueryRow(ctx, `
		INSERT INTO sample_orders (reseller_shop_id, supplier_listing_id, quantity, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id, reseller_shop_id, supplier_listing_id, status, quantity, COALESCE(notes,''), COALESCE(tracking_number,''), created_at
	`, resellerShopID, listingID, quantity, notes).Scan(
		&so.ID, &so.ResellerShopID, &so.SupplierListingID, &so.Status, &so.Quantity, &so.Notes, &so.TrackingNumber, &so.CreatedAt)
	return &so, err
}

func (s *Service) ListSampleOrders(ctx context.Context, shopID, role string) ([]SampleOrder, error) {
	var query string
	if role == "reseller" {
		query = `SELECT so.id, so.reseller_shop_id, so.supplier_listing_id, so.status, so.quantity,
				COALESCE(so.notes,''), COALESCE(so.tracking_number,''), so.created_at, COALESCE(sl.title,'')
			FROM sample_orders so LEFT JOIN supplier_listings sl ON sl.id = so.supplier_listing_id
			WHERE so.reseller_shop_id = $1 ORDER BY so.created_at DESC`
	} else {
		query = `SELECT so.id, so.reseller_shop_id, so.supplier_listing_id, so.status, so.quantity,
				COALESCE(so.notes,''), COALESCE(so.tracking_number,''), so.created_at, COALESCE(sl.title,'')
			FROM sample_orders so
			JOIN supplier_listings sl ON sl.id = so.supplier_listing_id AND sl.supplier_shop_id = $1
			ORDER BY so.created_at DESC`
	}
	rows, err := s.db.Query(ctx, query, shopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []SampleOrder
	for rows.Next() {
		var so SampleOrder
		if err := rows.Scan(&so.ID, &so.ResellerShopID, &so.SupplierListingID, &so.Status, &so.Quantity, &so.Notes, &so.TrackingNumber, &so.CreatedAt, &so.ListingTitle); err != nil {
			return nil, err
		}
		orders = append(orders, so)
	}
	return orders, nil
}

func (s *Service) UpdateSampleOrder(ctx context.Context, sampleID, status, tracking string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE sample_orders SET status = $1, tracking_number = COALESCE(NULLIF($2,''), tracking_number), updated_at = NOW()
		WHERE id = $3
	`, status, tracking, sampleID)
	return err
}

// ===== DEALS =====

type Deal struct {
	ID                string    `json:"id"`
	SupplierShopID    string    `json:"supplier_shop_id"`
	SupplierListingID *string   `json:"supplier_listing_id,omitempty"`
	Title             string    `json:"title"`
	DiscountType      string    `json:"discount_type"`
	DiscountValue     float64   `json:"discount_value"`
	StartsAt          time.Time `json:"starts_at"`
	EndsAt            time.Time `json:"ends_at"`
	MaxUses           int       `json:"max_uses"`
	CurrentUses       int       `json:"current_uses"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
}

func (s *Service) CreateDeal(ctx context.Context, supplierShopID string, deal Deal) (*Deal, error) {
	var d Deal
	err := s.db.QueryRow(ctx, `
		INSERT INTO deals (supplier_shop_id, supplier_listing_id, title, discount_type, discount_value, starts_at, ends_at, max_uses, target_reseller_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, supplier_shop_id, supplier_listing_id, title, discount_type, discount_value, starts_at, ends_at, max_uses, current_uses, is_active, created_at
	`, supplierShopID, deal.SupplierListingID, deal.Title, deal.DiscountType, deal.DiscountValue, deal.StartsAt, deal.EndsAt, deal.MaxUses, nil).Scan(
		&d.ID, &d.SupplierShopID, &d.SupplierListingID, &d.Title, &d.DiscountType, &d.DiscountValue, &d.StartsAt, &d.EndsAt, &d.MaxUses, &d.CurrentUses, &d.IsActive, &d.CreatedAt)
	return &d, err
}

func (s *Service) ListDeals(ctx context.Context, shopID, role string) ([]Deal, error) {
	var query string
	if role == "supplier" {
		query = `SELECT id, supplier_shop_id, supplier_listing_id, title, discount_type, discount_value, starts_at, ends_at, max_uses, current_uses, is_active, created_at
			FROM deals WHERE supplier_shop_id = $1 ORDER BY created_at DESC`
	} else {
		query = `SELECT DISTINCT d.id, d.supplier_shop_id, d.supplier_listing_id, d.title, d.discount_type, d.discount_value, d.starts_at, d.ends_at, d.max_uses, d.current_uses, d.is_active, d.created_at
			FROM deals d
			JOIN supplier_listings sl ON sl.supplier_shop_id = d.supplier_shop_id AND sl.status = 'active'
			JOIN reseller_imports ri ON ri.supplier_listing_id = sl.id AND ri.reseller_shop_id = $1
			WHERE d.is_active = TRUE AND d.ends_at > NOW() AND (d.target_reseller_id IS NULL OR d.target_reseller_id = $1)
			ORDER BY d.created_at DESC`
	}
	rows, err := s.db.Query(ctx, query, shopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deals []Deal
	for rows.Next() {
		var d Deal
		if err := rows.Scan(&d.ID, &d.SupplierShopID, &d.SupplierListingID, &d.Title, &d.DiscountType, &d.DiscountValue, &d.StartsAt, &d.EndsAt, &d.MaxUses, &d.CurrentUses, &d.IsActive, &d.CreatedAt); err != nil {
			return nil, err
		}
		deals = append(deals, d)
	}
	return deals, nil
}

// ===== PRODUCT PERFORMANCE =====

type ProductPerformance struct {
	ListingID     string  `json:"listing_id"`
	Title         string  `json:"title"`
	TotalOrders   int     `json:"total_orders"`
	TotalRevenue  float64 `json:"total_revenue"`
	TotalUnits    int     `json:"total_units"`
	AvgOrderValue float64 `json:"avg_order_value"`
}

func (s *Service) GetProductPerformance(ctx context.Context, shopID, role string) ([]ProductPerformance, error) {
	var query string
	if role == "supplier" {
		query = `SELECT sl.id, sl.title, COUNT(DISTINCT ro.id), COALESCE(SUM(roi.wholesale_unit_price * roi.quantity), 0),
				COALESCE(SUM(roi.quantity), 0), COALESCE(AVG(roi.wholesale_unit_price * roi.quantity), 0)
			FROM supplier_listings sl
			LEFT JOIN supplier_listing_variants slv ON slv.listing_id = sl.id
			LEFT JOIN product_links pl ON pl.supplier_variant_id = slv.shopify_variant_id AND pl.supplier_shop_id = $1
			LEFT JOIN routed_order_items roi ON roi.supplier_variant_id = pl.supplier_variant_id
			LEFT JOIN routed_orders ro ON ro.id = roi.routed_order_id
			WHERE sl.supplier_shop_id = $1
			GROUP BY sl.id, sl.title ORDER BY COALESCE(SUM(roi.wholesale_unit_price * roi.quantity), 0) DESC LIMIT 20`
	} else {
		query = `SELECT sl.id, sl.title, COUNT(DISTINCT ro.id), COALESCE(SUM(roi.wholesale_unit_price * roi.quantity), 0),
				COALESCE(SUM(roi.quantity), 0), COALESCE(AVG(roi.wholesale_unit_price * roi.quantity), 0)
			FROM reseller_imports ri
			JOIN supplier_listings sl ON sl.id = ri.supplier_listing_id
			LEFT JOIN product_links pl ON pl.import_id = ri.id
			LEFT JOIN routed_order_items roi ON roi.reseller_variant_id = pl.reseller_variant_id
			LEFT JOIN routed_orders ro ON ro.id = roi.routed_order_id AND ro.reseller_shop_id = $1
			WHERE ri.reseller_shop_id = $1
			GROUP BY sl.id, sl.title ORDER BY COALESCE(SUM(roi.wholesale_unit_price * roi.quantity), 0) DESC LIMIT 20`
	}
	rows, err := s.db.Query(ctx, query, shopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var perfs []ProductPerformance
	for rows.Next() {
		var p ProductPerformance
		if err := rows.Scan(&p.ListingID, &p.Title, &p.TotalOrders, &p.TotalRevenue, &p.TotalUnits, &p.AvgOrderValue); err != nil {
			return nil, err
		}
		perfs = append(perfs, p)
	}
	return perfs, nil
}

// ===== EXPORT (CSV generation) =====

func (s *Service) ExportOrders(ctx context.Context, shopID, role string) ([]byte, error) {
	shopColumn := "reseller_shop_id"
	if role == "supplier" {
		shopColumn = "supplier_shop_id"
	}
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT COALESCE(reseller_order_number,''), status, COALESCE(customer_shipping_name,''),
			COALESCE(total_wholesale_amount,0), COALESCE(currency,'USD'), created_at
		FROM routed_orders WHERE %s = $1 ORDER BY created_at DESC
	`, shopColumn), shopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	csv := "Order Number,Status,Customer,Amount,Currency,Date\n"
	for rows.Next() {
		var orderNum, status, customer, currency string
		var amount float64
		var date time.Time
		rows.Scan(&orderNum, &status, &customer, &amount, &currency, &date)
		csv += fmt.Sprintf("%s,%s,%s,%.2f,%s,%s\n", orderNum, status, customer, amount, currency, date.Format("2006-01-02"))
	}
	return []byte(csv), nil
}

// Suppress unused import warning
var _ = json.Marshal
