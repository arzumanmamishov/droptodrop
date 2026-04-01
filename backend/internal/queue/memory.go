package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// MemoryClient is an in-memory queue that processes jobs via goroutines.
// It has the same interface as the Redis Client but doesn't need Redis.
type MemoryClient struct {
	logger   zerolog.Logger
	handlers map[string]func(context.Context, json.RawMessage) error
}

// NewMemoryClient creates an in-memory queue client.
func NewMemoryClient(logger zerolog.Logger) *MemoryClient {
	return &MemoryClient{
		logger:   logger,
		handlers: make(map[string]func(context.Context, json.RawMessage) error),
	}
}

// RegisterHandler registers a job handler for a given job type.
func (c *MemoryClient) RegisterHandler(jobType string, handler func(context.Context, json.RawMessage) error) {
	c.handlers[jobType] = handler
}

// Enqueue processes the job immediately in a goroutine.
func (c *MemoryClient) Enqueue(ctx context.Context, queueName, jobType string, payload interface{}, maxRetry int) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	jobID := uuid.New().String()

	handler, ok := c.handlers[jobType]
	if !ok {
		c.logger.Warn().Str("type", jobType).Msg("no handler for job type, skipping")
		return jobID, nil
	}

	c.logger.Info().Str("job_id", jobID).Str("type", jobType).Str("queue", queueName).Msg("job enqueued (inline)")

	// Process in background goroutine
	go func() {
		bgCtx := context.Background()
		for attempt := 0; attempt <= maxRetry; attempt++ {
			err := handler(bgCtx, data)
			if err == nil {
				c.logger.Info().Str("job_id", jobID).Str("type", jobType).Msg("job completed")
				return
			}
			c.logger.Error().Err(err).Str("job_id", jobID).Int("attempt", attempt+1).Msg("job failed")
			if attempt < maxRetry {
				time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
			}
		}
		c.logger.Error().Str("job_id", jobID).Str("type", jobType).Msg("job failed permanently")
	}()

	return jobID, nil
}

// Close is a no-op for memory client.
func (c *MemoryClient) Close() error { return nil }

// Ping is a no-op for memory client.
func (c *MemoryClient) Ping(ctx context.Context) error { return nil }

// Set stores a value (no-op for memory queue, use for interface compatibility).
func (c *MemoryClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return nil
}

// Get retrieves a value (no-op for memory queue).
func (c *MemoryClient) Get(ctx context.Context, key string, dest interface{}) error {
	return fmt.Errorf("not found")
}
