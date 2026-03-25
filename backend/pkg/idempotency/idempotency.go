package idempotency

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store manages idempotency keys in Redis.
type Store struct {
	rdb *redis.Client
}

// NewStore creates a new idempotency store.
func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

// Check returns true if this key has been seen before (duplicate).
// If not seen, it sets the key with a TTL, returning false (not duplicate).
func (s *Store) Check(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	redisKey := fmt.Sprintf("idempotency:%s", key)
	set, err := s.rdb.SetNX(ctx, redisKey, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("check idempotency key: %w", err)
	}
	// SetNX returns true if key was set (new), false if already exists (duplicate)
	return !set, nil
}

// GenerateKey creates a deterministic key from components.
func GenerateKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte(":"))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
