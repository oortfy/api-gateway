package middleware

import (
	"net/http"
	"sync"
	"time"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
)

// RateLimiter represents a rate limiting middleware
type RateLimiter struct {
	limits       map[string]config.RateLimitConfig
	buckets      map[string]map[string]*tokenBucket
	bucketsMutex sync.RWMutex
	log          logger.Logger
}

// tokenBucket implements the token bucket algorithm for rate limiting
type tokenBucket struct {
	tokens         float64
	maxTokens      float64
	refillRate     float64
	lastRefillTime time.Time
	mutex          sync.Mutex
}

// NewRateLimiter creates a new rate limiting middleware
func NewRateLimiter(log logger.Logger) *RateLimiter {
	return &RateLimiter{
		limits:  make(map[string]config.RateLimitConfig),
		buckets: make(map[string]map[string]*tokenBucket),
		log:     log,
	}
}

// AddLimit adds a rate limit for a specific path
func (rl *RateLimiter) AddLimit(path string, limit config.RateLimitConfig) {
	rl.limits[path] = limit
	rl.buckets[path] = make(map[string]*tokenBucket)
}

// getBucket gets or creates a token bucket for a client
func (rl *RateLimiter) getBucket(path, clientID string) *tokenBucket {
	rl.bucketsMutex.RLock()
	bucket, exists := rl.buckets[path][clientID]
	rl.bucketsMutex.RUnlock()

	if exists {
		return bucket
	}

	// Create a new bucket if it doesn't exist
	rl.bucketsMutex.Lock()
	defer rl.bucketsMutex.Unlock()

	// Check again to avoid race conditions
	if bucket, exists = rl.buckets[path][clientID]; exists {
		return bucket
	}

	limit, exists := rl.limits[path]
	if !exists {
		// If no specific limit is set for this path, create a default bucket
		// that doesn't actually rate limit (high limit)
		bucket = &tokenBucket{
			tokens:         1000,
			maxTokens:      1000,
			refillRate:     1000,
			lastRefillTime: time.Now(),
		}
	} else {
		// Calculate tokens per second based on the limit
		var tokensPerSecond float64
		switch limit.Period {
		case "second":
			tokensPerSecond = float64(limit.Requests)
		case "minute":
			tokensPerSecond = float64(limit.Requests) / 60
		case "hour":
			tokensPerSecond = float64(limit.Requests) / 3600
		case "day":
			tokensPerSecond = float64(limit.Requests) / 86400
		default:
			tokensPerSecond = float64(limit.Requests) / 60 // Default to minute
		}

		bucket = &tokenBucket{
			tokens:         float64(limit.Requests),
			maxTokens:      float64(limit.Requests),
			refillRate:     tokensPerSecond,
			lastRefillTime: time.Now(),
		}
	}

	rl.buckets[path][clientID] = bucket
	return bucket
}

// RateLimit middleware applies rate limiting to requests
func (rl *RateLimiter) RateLimit(next http.Handler, route config.Route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting if not configured for this route
		if route.RateLimit == nil || route.RateLimit.Requests == 0 {
			next.ServeHTTP(w, r)
			return
		}

		// Use client IP as identifier for rate limiting
		// In production, you might want to use a more sophisticated method
		// like API keys or user IDs if available
		clientID := r.RemoteAddr

		// Get the bucket for this client
		bucket := rl.getBucket(route.Path, clientID)

		// Try to consume a token
		if allowed := rl.tryConsume(bucket); !allowed {
			rl.log.Debug("Rate limit exceeded",
				logger.String("path", r.URL.Path),
				logger.String("method", r.Method),
				logger.String("client", clientID),
			)

			w.Header().Set("Retry-After", "1") // Suggest retry after 1 second
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Continue to the next handler
		next.ServeHTTP(w, r)
	})
}

// tryConsume attempts to consume a token from the bucket
func (rl *RateLimiter) tryConsume(bucket *tokenBucket) bool {
	bucket.mutex.Lock()
	defer bucket.mutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(bucket.lastRefillTime).Seconds()

	// Refill the bucket based on time elapsed
	bucket.tokens = bucket.tokens + (elapsed * bucket.refillRate)
	if bucket.tokens > bucket.maxTokens {
		bucket.tokens = bucket.maxTokens
	}

	bucket.lastRefillTime = now

	// Check if we can consume a token
	if bucket.tokens < 1 {
		return false
	}

	// Consume a token
	bucket.tokens--
	return true
}
