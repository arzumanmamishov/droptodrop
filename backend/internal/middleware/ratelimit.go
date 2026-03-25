package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type rateLimiterStore struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
}

func newRateLimiterStore(rps int, burst int) *rateLimiterStore {
	return &rateLimiterStore{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
}

func (s *rateLimiterStore) getLimiter(key string) *rate.Limiter {
	s.mu.RLock()
	limiter, exists := s.limiters[key]
	s.mu.RUnlock()
	if exists {
		return limiter
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = s.limiters[key]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(s.rps, s.burst)
	s.limiters[key] = limiter
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
