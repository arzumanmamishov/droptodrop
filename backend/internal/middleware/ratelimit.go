package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

type rateLimiterStore struct {
	mu       sync.RWMutex
	limiters map[string]*limiterEntry
	rps      rate.Limit
	burst    int
}

const cleanupInterval = 1 * time.Minute
const entryTTL = 10 * time.Minute

func newRateLimiterStore(rps int, burst int) *rateLimiterStore {
	s := &rateLimiterStore{
		limiters: make(map[string]*limiterEntry),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
	go s.cleanupLoop()
	return s
}

func (s *rateLimiterStore) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		s.removeStale()
	}
}

func (s *rateLimiterStore) removeStale() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, entry := range s.limiters {
		if now.Sub(entry.lastAccess) > entryTTL {
			delete(s.limiters, key)
		}
	}
}

func (s *rateLimiterStore) getLimiter(key string) *rate.Limiter {
	now := time.Now()

	s.mu.RLock()
	entry, exists := s.limiters[key]
	s.mu.RUnlock()
	if exists {
		s.mu.Lock()
		entry.lastAccess = now
		s.mu.Unlock()
		return entry.limiter
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, exists = s.limiters[key]; exists {
		entry.lastAccess = now
		return entry.limiter
	}

	limiter := rate.NewLimiter(s.rps, s.burst)
	s.limiters[key] = &limiterEntry{
		limiter:    limiter,
		lastAccess: now,
	}
	return limiter
}

// RateLimit returns a middleware that rate-limits requests per shop.
func RateLimit(rps, burst int) gin.HandlerFunc {
	store := newRateLimiterStore(rps, burst)

	return func(c *gin.Context) {
		// Key by shop_id if authenticated, otherwise by IP
		key := c.ClientIP()
		if shopID, exists := c.Get("shop_id"); exists {
			key = shopID.(string)
		}

		limiter := store.getLimiter(key)
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}

		c.Next()
	}
}
