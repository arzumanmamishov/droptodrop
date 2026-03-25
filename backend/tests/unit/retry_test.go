package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/droptodrop/droptodrop/pkg/retry"
)

func TestRetry_SucceedsImmediately(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), retry.Config{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Millisecond,
	}, func() error {
		calls++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestRetry_SucceedsAfterRetries(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), retry.Config{
		MaxAttempts: 5,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Millisecond,
	}, func() error {
		calls++
		if calls < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestRetry_ExhaustsRetries(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), retry.Config{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Millisecond,
	}, func() error {
		calls++
		return errors.New("persistent error")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max retries exceeded")
	assert.Equal(t, 3, calls)
}

func TestRetry_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := retry.Do(ctx, retry.Config{
		MaxAttempts: 5,
		BaseDelay:   time.Second,
		MaxDelay:    time.Second,
	}, func() error {
		return errors.New("error")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}

func TestRetry_DefaultConfig(t *testing.T) {
	cfg := retry.DefaultConfig()
	assert.Equal(t, 3, cfg.MaxAttempts)
	assert.Equal(t, time.Second, cfg.BaseDelay)
	assert.Equal(t, 30*time.Second, cfg.MaxDelay)
}
