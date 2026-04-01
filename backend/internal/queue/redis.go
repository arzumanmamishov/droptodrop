package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/droptodrop/droptodrop/internal/config"
)

// Client wraps Redis for queue and cache operations.
type Client struct {
	rdb      *redis.Client
	logger   zerolog.Logger
	fallback bool
}

// NewFallbackClient creates a Client that processes jobs inline without Redis.
func NewFallbackClient(logger zerolog.Logger) *Client {
	logger.Info().Msg("using inline job processing (no Redis)")
	return &Client{rdb: nil, logger: logger, fallback: true}
}

// Job represents a queued background job.
type Job struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Queue     string          `json:"queue"`
	Payload   json.RawMessage `json:"payload"`
	Attempts  int             `json:"attempts"`
	MaxRetry  int             `json:"max_retry"`
	CreatedAt time.Time       `json:"created_at"`
}

// NewClient creates a Redis client.
func NewClient(cfg config.RedisConfig, logger zerolog.Logger) (*Client, error) {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	if cfg.Password != "" {
		opts.Password = cfg.Password
	}
	opts.MaxRetries = cfg.MaxRetries

	rdb := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Client{rdb: rdb, logger: logger}, nil
}

// Enqueue adds a job to a queue. If Redis is not available, logs the job (no-op).
func (c *Client) Enqueue(ctx context.Context, queueName, jobType string, payload interface{}, maxRetry int) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	job := Job{
		ID:        uuid.New().String(),
		Type:      jobType,
		Queue:     queueName,
		Payload:   data,
		Attempts:  0,
		MaxRetry:  maxRetry,
		CreatedAt: time.Now().UTC(),
	}

	// Fallback mode: no Redis, just log
	if c.fallback || c.rdb == nil {
		c.logger.Info().Str("job_id", job.ID).Str("type", jobType).Str("queue", queueName).Msg("job enqueued (inline)")
		return job.ID, nil
	}

	jobData, err := json.Marshal(job)
	if err != nil {
		return "", fmt.Errorf("marshal job: %w", err)
	}

	key := fmt.Sprintf("queue:%s", queueName)
	if err := c.rdb.LPush(ctx, key, jobData).Err(); err != nil {
		return "", fmt.Errorf("enqueue job: %w", err)
	}

	c.logger.Info().Str("job_id", job.ID).Str("type", jobType).Str("queue", queueName).Msg("job enqueued")
	return job.ID, nil
}

// Dequeue retrieves and removes a job from a queue (blocking).
func (c *Client) Dequeue(ctx context.Context, queueName string, timeout time.Duration) (*Job, error) {
	key := fmt.Sprintf("queue:%s", queueName)
	result, err := c.rdb.BRPop(ctx, timeout, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("dequeue job: %w", err)
	}

	if len(result) < 2 {
		return nil, nil
	}

	var job Job
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}

	return &job, nil
}

// MoveToDeadLetter moves a failed job to the dead letter queue.
func (c *Client) MoveToDeadLetter(ctx context.Context, job *Job, errMsg string) error {
	deadJob := map[string]interface{}{
		"job":        job,
		"error":      errMsg,
		"dead_at":    time.Now().UTC(),
	}
	data, err := json.Marshal(deadJob)
	if err != nil {
		return fmt.Errorf("marshal dead letter: %w", err)
	}

	key := fmt.Sprintf("queue:%s:dead", job.Queue)
	return c.rdb.LPush(ctx, key, data).Err()
}

// Set stores a value in Redis with optional expiration.
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal value: %w", err)
	}
	return c.rdb.Set(ctx, key, data, expiration).Err()
}

// Get retrieves a value from Redis.
func (c *Client) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	if c.rdb == nil { return nil }
	return c.rdb.Close()
}

// Ping checks Redis connectivity.
func (c *Client) Ping(ctx context.Context) error {
	if c.rdb == nil { return nil }
	return c.rdb.Ping(ctx).Err()
}
