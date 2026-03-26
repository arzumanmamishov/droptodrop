package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Service provides audit logging capabilities.
type Service struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
}

// NewService creates an audit service.
func NewService(db *pgxpool.Pool, logger zerolog.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// Log records an audit event.
func (s *Service) Log(ctx context.Context, shopID, actorType, actorID, action, resourceType, resourceID string, details interface{}, outcome, errorPayload string) {
	detailsJSON := []byte("{}")
	if details != nil {
		if d, err := json.Marshal(details); err == nil {
			detailsJSON = d
		}
	}

	_, err := s.db.Exec(ctx, `
		INSERT INTO audit_logs (shop_id, actor_type, actor_id, action, resource_type, resource_id, details, outcome, error_payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, shopID, actorType, actorID, action, resourceType, resourceID, detailsJSON, outcome, errorPayload)

	if err != nil {
		s.logger.Error().Err(err).Str("action", action).Msg("failed to write audit log")
	}
}

// Entry represents an audit log entry for API responses.
type Entry struct {
	ID           string          `json:"id"`
	ShopID       string          `json:"shop_id"`
	ActorType    string          `json:"actor_type"`
	ActorID      string          `json:"actor_id"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Details      json.RawMessage `json:"details"`
	Outcome      string          `json:"outcome"`
	ErrorPayload string          `json:"error_payload,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// List returns audit entries for a shop with pagination.
func (s *Service) List(ctx context.Context, shopID string, limit, offset int) ([]Entry, int, error) {
	var total int
	err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE shop_id = $1`, shopID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, shop_id, actor_type, actor_id, action, resource_type, resource_id, details, outcome, error_payload, created_at
		FROM audit_logs
		WHERE shop_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, shopID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.ShopID, &e.ActorType, &e.ActorID, &e.Action, &e.ResourceType, &e.ResourceID, &e.Details, &e.Outcome, &e.ErrorPayload, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}

	return entries, total, nil
}
