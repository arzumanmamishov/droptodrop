package billing

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Handler provides billing-related endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a billing handler.
func NewHandler(db *pgxpool.Pool, logger zerolog.Logger) *Handler {
	return &Handler{svc: NewService(db, logger)}
}

// GetSvc returns the underlying billing service.
func (h *Handler) GetSvc() *Service {
	return h.svc
}
