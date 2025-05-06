package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
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
	rl.log.Info("Rate limit added",
		logger.String("path", path),
		logger.Int("requests", limit.Requests),
		logger.String("period", limit.Period))
}

// getBucket gets or creates a token bucket for a client
func (rl *RateLimiter) getBucket(path, clientID string) *tokenBucket {
	rl.bucketsMutex.RLock()
	pathBuckets, pathExists := rl.buckets[path]
	if !pathExists {
		rl.bucketsMutex.RUnlock()
		return nil // Path not configured for rate limiting
	}

	bucket, clientExists := pathBuckets[clientID]
	rl.bucketsMutex.RUnlock()

	if clientExists {
		return bucket
	}

	// Create a new bucket if it doesn't exist
	rl.bucketsMutex.Lock()
	defer rl.bucketsMutex.Unlock()

	// Check again to avoid race conditions
	if pathBuckets, pathExists = rl.buckets[path]; !pathExists {
		return nil // Path not configured for rate limiting
	}

	if bucket, clientExists = pathBuckets[clientID]; clientExists {
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

		rl.log.Debug("New rate limit bucket created",
			logger.String("path", path),
			logger.String("client", clientID),
			logger.Int("max_tokens", int(limit.Requests)),
			logger.String("refill_rate", fmt.Sprintf("%.4f tokens/sec", tokensPerSecond)))
	}

	rl.buckets[path][clientID] = bucket
	return bucket
}

// getClientIP extracts the client IP from the request, handling common proxy headers
func (rl *RateLimiter) getClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header (common in reverse proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		// The leftmost IP is the original client
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			return clientIP
		}
	}

	// Check for X-Real-IP header (used by some proxies)
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}

	// Extract IP from RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If we can't split, use the whole RemoteAddr
		ip = r.RemoteAddr
	}

	return ip
}

// RateLimit middleware applies rate limiting to requests
func (rl *RateLimiter) RateLimit(next http.Handler, route config.Route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting if not configured for this route
		if route.Middlewares.RateLimit == nil || route.Middlewares.RateLimit.Requests == 0 {
			next.ServeHTTP(w, r)
			return
		}

		// Get a unique identifier for this client
		// In addition to IP, we can use API key or auth info if available
		clientID := rl.getClientIP(r)

		// Add auth information if available for per-user rate limiting
		if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
			clientID = apiKey // Use API key as identifier if present
		} else if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			clientID = authHeader // Use auth token as identifier
		}

		pathKey := route.Path
		rl.log.Debug("Rate limit check",
			logger.String("path", r.URL.Path),
			logger.String("pathKey", pathKey),
			logger.String("clientID", clientID))

		// Get the bucket for this client
		bucket := rl.getBucket(pathKey, clientID)
		if bucket == nil {
			rl.log.Warn("No rate limit bucket found for path",
				logger.String("path", pathKey))
			next.ServeHTTP(w, r)
			return
		}

		// Try to consume a token
		if allowed := rl.tryConsume(bucket); !allowed {
			rl.log.Info("Rate limit exceeded",
				logger.String("path", r.URL.Path),
				logger.String("method", r.Method),
				logger.String("client", clientID),
			)

			w.Header().Set("Retry-After", "60") // Suggest retry after period
			w.Header().Set("X-RateLimit-Limit", "2")
			w.Header().Set("X-RateLimit-Remaining", "0")
			http.Error(w, "Rate limit exceeded. Try again later.", http.StatusTooManyRequests)
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
