package retry

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Config holds retry configuration.
type Config struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultConfig returns a sensible default retry config.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
	}
}

// Do executes fn with exponential backoff retry.
func Do(ctx context.Context, cfg Config, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled: %w", err)
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if attempt < cfg.MaxAttempts-1 {
			delay := time.Duration(math.Pow(2, float64(attempt))) * cfg.BaseDelay
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}
