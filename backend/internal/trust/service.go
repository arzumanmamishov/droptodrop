package trust

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type SupplierStats struct {
	SupplierShopID    string    `json:"supplier_shop_id"`
	TotalOrders       int       `json:"total_orders"`
	FulfilledOrders   int       `json:"fulfilled_orders"`
	CancelledOrders   int       `json:"cancelled_orders"`
	DisputedOrders    int       `json:"disputed_orders"`
	AvgFulfillHours   float64   `json:"avg_fulfillment_hours"`
	FulfillmentRate   float64   `json:"fulfillment_rate"`
	DisputeRate       float64   `json:"dispute_rate"`
	Score             int       `json:"score"`
	Label             string    `json:"label"`
	LastUpdated       time.Time `json:"last_updated"`
}

type Service struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
}

func NewService(db *pgxpool.Pool, logger zerolog.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// GetStats returns supplier stats, computing if not exists.
func (s *Service) GetStats(ctx context.Context, supplierShopID string) (*SupplierStats, error) {
	var stats SupplierStats
	err := s.db.QueryRow(ctx, `
		SELECT supplier_shop_id, total_orders, fulfilled_orders, cancelled_orders, disputed_orders,
			avg_fulfillment_hours, fulfillment_rate, dispute_rate, score, label, last_updated
		FROM supplier_stats WHERE supplier_shop_id = $1
	`, supplierShopID).Scan(&stats.SupplierShopID, &stats.TotalOrders, &stats.FulfilledOrders,
		&stats.CancelledOrders, &stats.DisputedOrders, &stats.AvgFulfillHours,
		&stats.FulfillmentRate, &stats.DisputeRate, &stats.Score, &stats.Label, &stats.LastUpdated)
	if err != nil {
		// Stats don't exist yet, compute them
		return s.RecalculateStats(ctx, supplierShopID)
	}
	return &stats, nil
}

// RecalculateStats recomputes supplier stats from actual order data.
func (s *Service) RecalculateStats(ctx context.Context, supplierShopID string) (*SupplierStats, error) {
	stats := &SupplierStats{SupplierShopID: supplierShopID}

	// Total orders received
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM routed_orders WHERE supplier_shop_id = $1`, supplierShopID).Scan(&stats.TotalOrders)

	// Fulfilled orders
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM routed_orders WHERE supplier_shop_id = $1 AND status = 'fulfilled'`, supplierShopID).Scan(&stats.FulfilledOrders)

	// Cancelled/rejected orders
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM routed_orders WHERE supplier_shop_id = $1 AND status IN ('rejected', 'cancelled')`, supplierShopID).Scan(&stats.CancelledOrders)

	// Disputed orders
	s.db.QueryRow(ctx, `SELECT COUNT(DISTINCT routed_order_id) FROM disputes WHERE reporter_shop_id != $1 AND routed_order_id IN (SELECT id FROM routed_orders WHERE supplier_shop_id = $1)`, supplierShopID).Scan(&stats.DisputedOrders)

	// Average fulfillment time (hours between created_at and fulfilled_at)
	s.db.QueryRow(ctx, `
		SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (fulfilled_at - created_at)) / 3600), 0)
		FROM routed_orders WHERE supplier_shop_id = $1 AND status = 'fulfilled' AND fulfilled_at IS NOT NULL
	`, supplierShopID).Scan(&stats.AvgFulfillHours)

	// Calculate rates
	if stats.TotalOrders > 0 {
		stats.FulfillmentRate = float64(stats.FulfilledOrders) / float64(stats.TotalOrders) * 100
		stats.DisputeRate = float64(stats.DisputedOrders) / float64(stats.TotalOrders) * 100
	}

	// Calculate score (0-100)
	stats.Score = s.calculateScore(stats)
	stats.Label = s.getLabel(stats.Score)
	stats.LastUpdated = time.Now()

	// Upsert stats
	_, err := s.db.Exec(ctx, `
		INSERT INTO supplier_stats (supplier_shop_id, total_orders, fulfilled_orders, cancelled_orders, disputed_orders,
			avg_fulfillment_hours, fulfillment_rate, dispute_rate, score, label, last_updated)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (supplier_shop_id) DO UPDATE SET
			total_orders = $2, fulfilled_orders = $3, cancelled_orders = $4, disputed_orders = $5,
			avg_fulfillment_hours = $6, fulfillment_rate = $7, dispute_rate = $8, score = $9, label = $10, last_updated = $11
	`, supplierShopID, stats.TotalOrders, stats.FulfilledOrders, stats.CancelledOrders, stats.DisputedOrders,
		stats.AvgFulfillHours, stats.FulfillmentRate, stats.DisputeRate, stats.Score, stats.Label, stats.LastUpdated)
	if err != nil {
		s.logger.Warn().Err(err).Str("supplier", supplierShopID).Msg("failed to upsert supplier stats")
	}

	return stats, nil
}

// calculateScore computes a 0-100 supplier reliability score.
func (s *Service) calculateScore(stats *SupplierStats) int {
	if stats.TotalOrders == 0 {
		return 50 // New supplier, neutral score
	}

	// Fulfillment rate: 60% weight (0-60 points)
	fulfillScore := stats.FulfillmentRate * 0.6

	// Shipping speed: 20% weight (0-20 points)
	// Under 24h = 20, under 48h = 15, under 72h = 10, over = 5
	speedScore := 5.0
	if stats.AvgFulfillHours <= 24 {
		speedScore = 20
	} else if stats.AvgFulfillHours <= 48 {
		speedScore = 15
	} else if stats.AvgFulfillHours <= 72 {
		speedScore = 10
	}

	// Low dispute rate: 20% weight (0-20 points)
	disputeScore := 20.0
	if stats.DisputeRate > 0 {
		disputeScore = 20 - (stats.DisputeRate * 4) // -4 points per 1% dispute rate
		if disputeScore < 0 {
			disputeScore = 0
		}
	}

	total := int(fulfillScore + speedScore + disputeScore)
	if total > 100 {
		total = 100
	}
	if total < 0 {
		total = 0
	}
	return total
}

// getLabel returns a human-readable label for a score.
func (s *Service) getLabel(score int) string {
	switch {
	case score >= 90:
		return "Top Supplier"
	case score >= 75:
		return "Reliable"
	case score >= 50:
		return "Average"
	case score >= 30:
		return "Needs Improvement"
	default:
		return "Risky"
	}
}

// CheckAndEnforceRisk checks if a supplier should be warned or disabled.
func (s *Service) CheckAndEnforceRisk(ctx context.Context, supplierShopID string) (string, error) {
	stats, err := s.GetStats(ctx, supplierShopID)
	if err != nil {
		return "", err
	}

	if stats.TotalOrders < 5 {
		return "ok", nil // Not enough data
	}

	if stats.FulfillmentRate < 80 {
		// Disable listings
		_, err := s.db.Exec(ctx, `
			UPDATE supplier_listings SET status = 'paused'
			WHERE supplier_shop_id = $1 AND status = 'active'
		`, supplierShopID)
		if err != nil {
			s.logger.Error().Err(err).Msg("failed to disable supplier listings")
		}
		s.logger.Warn().Str("supplier", supplierShopID).Float64("rate", stats.FulfillmentRate).Msg("supplier listings disabled due to low fulfillment rate")
		return "disabled", nil
	}

	if stats.FulfillmentRate < 90 {
		s.logger.Warn().Str("supplier", supplierShopID).Float64("rate", stats.FulfillmentRate).Msg("supplier warned: low fulfillment rate")
		return "warned", nil
	}

	return "ok", nil
}

// IsVerified checks if a supplier is verified.
func (s *Service) IsVerified(ctx context.Context, supplierShopID string) bool {
	var verified bool
	s.db.QueryRow(ctx, `SELECT COALESCE(is_verified, FALSE) FROM supplier_profiles WHERE shop_id = $1`, supplierShopID).Scan(&verified)
	return verified
}
