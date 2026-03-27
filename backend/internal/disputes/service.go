package disputes

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Dispute represents a dispute filed against a routed order.
type Dispute struct {
	ID             string    `json:"id"`
	RoutedOrderID  string    `json:"routed_order_id"`
	ReporterShopID string    `json:"reporter_shop_id"`
	ReporterRole   string    `json:"reporter_role"`
	DisputeType    string    `json:"dispute_type"`
	Status         string    `json:"status"`
	Description    string    `json:"description"`
	Resolution     *string   `json:"resolution"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreateInput holds the fields needed to create a dispute.
type CreateInput struct {
	RoutedOrderID string `json:"routed_order_id" binding:"required"`
	DisputeType   string `json:"dispute_type" binding:"required"`
	Description   string `json:"description" binding:"required"`
}

// UpdateInput holds the fields that can be updated on a dispute.
type UpdateInput struct {
	Status     string  `json:"status"`
	Resolution *string `json:"resolution"`
}

// Service handles dispute operations.
type Service struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
}

// NewService creates a new disputes service.
func NewService(db *pgxpool.Pool, logger zerolog.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// Create creates a new dispute for a routed order.
func (s *Service) Create(ctx context.Context, shopID, role string, input CreateInput) (*Dispute, error) {
	// Verify the shop is part of this routed order
	var exists bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM routed_orders
			WHERE id = $1 AND (reseller_shop_id = $2 OR supplier_shop_id = $2)
		)
	`, input.RoutedOrderID, shopID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check order access: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("order not found or access denied")
	}

	d := &Dispute{}
	err = s.db.QueryRow(ctx, `
		INSERT INTO disputes (routed_order_id, reporter_shop_id, reporter_role, dispute_type, description)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, routed_order_id, reporter_shop_id, reporter_role, dispute_type, status, description, resolution, created_at, updated_at
	`, input.RoutedOrderID, shopID, role, input.DisputeType, input.Description).Scan(
		&d.ID, &d.RoutedOrderID, &d.ReporterShopID, &d.ReporterRole,
		&d.DisputeType, &d.Status, &d.Description, &d.Resolution,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create dispute: %w", err)
	}

	s.logger.Info().Str("dispute_id", d.ID).Str("order_id", input.RoutedOrderID).Msg("dispute created")
	return d, nil
}

// Get returns a single dispute by ID, scoped to the given shop.
func (s *Service) Get(ctx context.Context, disputeID, shopID string) (*Dispute, error) {
	d := &Dispute{}
	err := s.db.QueryRow(ctx, `
		SELECT d.id, d.routed_order_id, d.reporter_shop_id, d.reporter_role,
			d.dispute_type, d.status, d.description, d.resolution, d.created_at, d.updated_at
		FROM disputes d
		JOIN routed_orders ro ON ro.id = d.routed_order_id
		WHERE d.id = $1 AND (ro.reseller_shop_id = $2 OR ro.supplier_shop_id = $2)
	`, disputeID, shopID).Scan(
		&d.ID, &d.RoutedOrderID, &d.ReporterShopID, &d.ReporterRole,
		&d.DisputeType, &d.Status, &d.Description, &d.Resolution,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get dispute: %w", err)
	}
	return d, nil
}

// ListByShop returns disputes visible to a shop (as supplier or reseller on the order).
func (s *Service) ListByShop(ctx context.Context, shopID string, limit, offset int) ([]Dispute, int, error) {
	var total int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM disputes d
		JOIN routed_orders ro ON ro.id = d.routed_order_id
		WHERE ro.reseller_shop_id = $1 OR ro.supplier_shop_id = $1
	`, shopID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count disputes: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT d.id, d.routed_order_id, d.reporter_shop_id, d.reporter_role,
			d.dispute_type, d.status, d.description, d.resolution, d.created_at, d.updated_at
		FROM disputes d
		JOIN routed_orders ro ON ro.id = d.routed_order_id
		WHERE ro.reseller_shop_id = $1 OR ro.supplier_shop_id = $1
		ORDER BY d.created_at DESC
		LIMIT $2 OFFSET $3
	`, shopID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list disputes: %w", err)
	}
	defer rows.Close()

	var disputes []Dispute
	for rows.Next() {
		var d Dispute
		if err := rows.Scan(
			&d.ID, &d.RoutedOrderID, &d.ReporterShopID, &d.ReporterRole,
			&d.DisputeType, &d.Status, &d.Description, &d.Resolution,
			&d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan dispute: %w", err)
		}
		disputes = append(disputes, d)
	}

	return disputes, total, nil
}

// ListByOrder returns all disputes for a specific routed order.
func (s *Service) ListByOrder(ctx context.Context, orderID, shopID string) ([]Dispute, error) {
	rows, err := s.db.Query(ctx, `
		SELECT d.id, d.routed_order_id, d.reporter_shop_id, d.reporter_role,
			d.dispute_type, d.status, d.description, d.resolution, d.created_at, d.updated_at
		FROM disputes d
		JOIN routed_orders ro ON ro.id = d.routed_order_id
		WHERE d.routed_order_id = $1 AND (ro.reseller_shop_id = $2 OR ro.supplier_shop_id = $2)
		ORDER BY d.created_at DESC
	`, orderID, shopID)
	if err != nil {
		return nil, fmt.Errorf("list disputes by order: %w", err)
	}
	defer rows.Close()

	var disputes []Dispute
	for rows.Next() {
		var d Dispute
		if err := rows.Scan(
			&d.ID, &d.RoutedOrderID, &d.ReporterShopID, &d.ReporterRole,
			&d.DisputeType, &d.Status, &d.Description, &d.Resolution,
			&d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dispute: %w", err)
		}
		disputes = append(disputes, d)
	}

	return disputes, nil
}

// UpdateStatus updates a dispute's status and optionally its resolution text.
func (s *Service) UpdateStatus(ctx context.Context, disputeID, shopID string, input UpdateInput) (*Dispute, error) {
	d := &Dispute{}
	err := s.db.QueryRow(ctx, `
		UPDATE disputes SET
			status = COALESCE(NULLIF($3, ''), status),
			resolution = COALESCE($4, resolution),
			updated_at = NOW()
		FROM routed_orders ro
		WHERE disputes.id = $1
			AND disputes.routed_order_id = ro.id
			AND (ro.reseller_shop_id = $2 OR ro.supplier_shop_id = $2)
		RETURNING disputes.id, disputes.routed_order_id, disputes.reporter_shop_id, disputes.reporter_role,
			disputes.dispute_type, disputes.status, disputes.description, disputes.resolution,
			disputes.created_at, disputes.updated_at
	`, disputeID, shopID, input.Status, input.Resolution).Scan(
		&d.ID, &d.RoutedOrderID, &d.ReporterShopID, &d.ReporterRole,
		&d.DisputeType, &d.Status, &d.Description, &d.Resolution,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update dispute: %w", err)
	}

	s.logger.Info().Str("dispute_id", d.ID).Str("status", d.Status).Msg("dispute updated")
	return d, nil
}
