package queue

import (
	"context"
	"time"
)

// QueueClient is the interface for queue operations.
// Both Redis and Memory clients implement this.
type QueueClient interface {
	Enqueue(ctx context.Context, queueName, jobType string, payload interface{}, maxRetry int) (string, error)
	Close() error
	Ping(ctx context.Context) error
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string, dest interface{}) error
}
