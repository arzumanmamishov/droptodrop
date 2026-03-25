package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/droptodrop/droptodrop/internal/queue"
)

// Handler provides health and readiness endpoints.
type Handler struct {
	db    *pgxpool.Pool
	redis *queue.Client
}

// NewHandler creates a health handler.
func NewHandler(db *pgxpool.Pool, redis *queue.Client) *Handler {
	return &Handler{db: db, redis: redis}
}

// Liveness returns 200 if the server is running.
func (h *Handler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readiness checks database and Redis connectivity.
func (h *Handler) Readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]string)
	healthy := true

	// Check database
	if err := h.db.Ping(ctx); err != nil {
		checks["database"] = "unhealthy: " + err.Error()
		healthy = false
	} else {
		checks["database"] = "ok"
	}

	// Check Redis
	if err := h.redis.Ping(ctx); err != nil {
		checks["redis"] = "unhealthy: " + err.Error()
		healthy = false
	} else {
		checks["redis"] = "ok"
	}

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, gin.H{
		"status": map[bool]string{true: "healthy", false: "unhealthy"}[healthy],
		"checks": checks,
	})
}
