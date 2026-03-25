package fulfillments

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/droptodrop/droptodrop/internal/audit"
	"github.com/droptodrop/droptodrop/internal/queue"
)

// FulfillmentEvent represents a fulfillment tracking event.
type FulfillmentEvent struct {
	ID                   string `json:"id"`
	RoutedOrderID        string `json:"routed_order_id"`
	ShopifyFulfillmentID *int64 `json:"shopify_fulfillment_id,omitempty"`
	TrackingNumber       string `json:"tracking_number"`
	TrackingURL          string `json:"tracking_url"`
	TrackingCompany      string `json:"tracking_company"`
	Status               string `json:"status"`
	SyncedToReseller     bool   `json:"synced_to_reseller"`
	SyncedAt             string `json:"synced_at,omitempty"`
	SyncError            string `json:"sync_error,omitempty"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
}

// AddTrackingInput is input for adding fulfillment tracking.
type AddTrackingInput struct {
	RoutedOrderID   string `json:"routed_order_id" binding:"required"`
	TrackingNumber  string `json:"tracking_number" binding:"required"`
	TrackingURL     string `json:"tracking_url"`
	TrackingCompany string `json:"tracking_company"`
}

// Service handles fulfillment operations.
type Service struct {
	db     *pgxpool.Pool
	queue  *queue.Client
	logger zerolog.Logger
	audit  *audit.Service
}

// NewService creates a fulfillment service.
func NewService(db *pgxpool.Pool, q *queue.Client, logger zerolog.Logger, auditSvc *audit.Service) *Service {
	return &Service{db: db, queue: q, logger: logger, audit: auditSvc}
}

// AddTracking adds tracking information and marks items as fulfilled.
func (s *Service) AddTracking(ctx context.Context, supplierShopID string, input AddTrackingInput) (*FulfillmentEvent, error) {
	// Verify the order belongs to this supplier and is accepted
	var orderStatus string
	err := s.db.QueryRow(ctx, `
		SELECT status FROM routed_orders WHERE id = $1 AND supplier_shop_id = $2
	`, input.RoutedOrderID, supplierShopID).Scan(&orderStatus)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}
	if orderStatus != "accepted" && orderStatus != "processing" && orderStatus != "partially_fulfilled" {
		return nil, fmt.Errorf("order must be accepted before fulfillment, current status: %s", orderStatus)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Create fulfillment event
	var event FulfillmentEvent
	err = tx.QueryRow(ctx, `
		INSERT INTO fulfillment_events (routed_order_id, tracking_number, tracking_url, tracking_company, status)
		VALUES ($1, $2, $3, $4, 'in_transit')
		RETURNING id, routed_order_id, tracking_number, COALESCE(tracking_url,''), COALESCE(tracking_company,''),
			status, synced_to_reseller, created_at, updated_at
	`, input.RoutedOrderID, input.TrackingNumber, input.TrackingURL, input.TrackingCompany,
	).Scan(&event.ID, &event.RoutedOrderID, &event.TrackingNumber, &event.TrackingURL,
		&event.TrackingCompany, &event.Status, &event.SyncedToReseller, &event.CreatedAt, &event.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create fulfillment event: %w", err)
	}

	// Update order items to fulfilled
	_, err = tx.Exec(ctx, `
		UPDATE routed_order_items SET fulfillment_status = 'fulfilled', fulfilled_quantity = quantity
		WHERE routed_order_id = $1 AND fulfillment_status = 'unfulfilled'
	`, input.RoutedOrderID)
	if err != nil {
		return nil, fmt.Errorf("update items: %w", err)
	}

	// Update order status
	_, err = tx.Exec(ctx, `
		UPDATE routed_orders SET status = 'fulfilled', fulfilled_at = NOW()
		WHERE id = $1
	`, input.RoutedOrderID)
	if err != nil {
		return nil, fmt.Errorf("update order status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// Queue sync to reseller
	_, err = s.queue.Enqueue(ctx, "fulfillments", "sync_to_reseller", map[string]string{
		"fulfillment_event_id": event.ID,
		"routed_order_id":     input.RoutedOrderID,
	}, 3)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to enqueue fulfillment sync")
	}

	s.audit.Log(ctx, supplierShopID, "merchant", supplierShopID, "fulfillment_added", "fulfillment_event", event.ID,
		map[string]string{"tracking": input.TrackingNumber, "order_id": input.RoutedOrderID}, "success", "")

	return &event, nil
}

// ListByOrder returns fulfillment events for a routed order.
func (s *Service) ListByOrder(ctx context.Context, routedOrderID string) ([]FulfillmentEvent, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, routed_order_id, shopify_fulfillment_id, tracking_number, COALESCE(tracking_url,''),
			COALESCE(tracking_company,''), status, synced_to_reseller, synced_at, COALESCE(sync_error,''),
			created_at, updated_at
		FROM fulfillment_events WHERE routed_order_id = $1 ORDER BY created_at
	`, routedOrderID)
	if err != nil {
		return nil, fmt.Errorf("list fulfillments: %w", err)
	}
	defer rows.Close()

	var events []FulfillmentEvent
	for rows.Next() {
		var e FulfillmentEvent
		if err := rows.Scan(&e.ID, &e.RoutedOrderID, &e.ShopifyFulfillmentID, &e.TrackingNumber,
			&e.TrackingURL, &e.TrackingCompany, &e.Status, &e.SyncedToReseller, &e.SyncedAt,
			&e.SyncError, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}

	return events, nil
}

// MarkSynced marks a fulfillment event as synced to the reseller.
func (s *Service) MarkSynced(ctx context.Context, eventID string, shopifyFulfillmentID int64) error {
	_, err := s.db.Exec(ctx, `
		UPDATE fulfillment_events SET synced_to_reseller = TRUE, synced_at = NOW(), shopify_fulfillment_id = $2
		WHERE id = $1
	`, eventID, shopifyFulfillmentID)
	return err
}

// MarkSyncFailed records a sync failure.
func (s *Service) MarkSyncFailed(ctx context.Context, eventID, errMsg string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE fulfillment_events SET sync_error = $2 WHERE id = $1
	`, eventID, errMsg)
	return err
}
