package handlers

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"api-gateway/internal/config"
)

// AuthMiddleware handles authentication
type AuthMiddleware struct {
	next http.Handler
	cfg  config.AuthConfig
}

func NewAuthMiddleware(next http.Handler, cfg config.AuthConfig) http.Handler {
	return &AuthMiddleware{next: next, cfg: cfg}
}

func (m *AuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check for API key first
	if apiKey := r.Header.Get(m.cfg.APIKeyHeader); apiKey != "" {
		if err := m.validateAPIKey(apiKey); err != nil {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}
		m.next.ServeHTTP(w, r)
		return
	}

	// Check for JWT
	if tokenStr := r.Header.Get(m.cfg.JWTHeader); tokenStr != "" {
		if err := m.validateJWT(tokenStr); err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		m.next.ServeHTTP(w, r)
		return
	}

	http.Error(w, "Authentication required", http.StatusUnauthorized)
}

func (m *AuthMiddleware) validateAPIKey(apiKey string) error {
	// Implement API key validation logic
	// This could involve calling an external service or checking against a database
	return nil
}

func (m *AuthMiddleware) validateJWT(tokenStr string) error {
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.cfg.JWTSecret), nil
	})

	if err != nil {
		return err
	}

	if !token.Valid {
		return fmt.Errorf("invalid token")
	}

	return nil
}

// RateLimitMiddleware handles rate limiting
type RateLimitMiddleware struct {
	next     http.Handler
	limiters sync.Map
	config   *config.RateLimitConfig
}

func NewRateLimitMiddleware(next http.Handler, cfg *config.RateLimitConfig) http.Handler {
	return &RateLimitMiddleware{
		next:   next,
		config: cfg,
	}
}

func (m *RateLimitMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.RemoteAddr
	if clientIP := r.Header.Get("X-Real-IP"); clientIP != "" {
		key = clientIP
	}

	// Simple token bucket implementation
	bucket, _ := m.limiters.LoadOrStore(key, &tokenBucket{
		tokens:     m.config.Requests,
		capacity:   m.config.Requests,
		lastRefill: time.Now(),
	})

	if !bucket.(*tokenBucket).allow() {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	m.next.ServeHTTP(w, r)
}

// tokenBucket implements a simple token bucket algorithm
type tokenBucket struct {
	sync.Mutex
	tokens     int
	capacity   int
	lastRefill time.Time
}

func (tb *tokenBucket) allow() bool {
	tb.Lock()
	defer tb.Unlock()

	now := time.Now()
	tb.refill(now)

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

func (tb *tokenBucket) refill(now time.Time) {
	elapsed := now.Sub(tb.lastRefill)
	tokensToAdd := int(elapsed.Seconds()) * tb.capacity
	if tokensToAdd > 0 {
		tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
		tb.lastRefill = now
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CircuitBreakerMiddleware implements the circuit breaker pattern
type CircuitBreakerMiddleware struct {
	next   http.Handler
	config *config.CircuitBreakerSettings
	state  struct {
		sync.RWMutex
		open      bool
		failures  int
		lastError time.Time
	}
}

func NewCircuitBreakerMiddleware(next http.Handler, cfg *config.CircuitBreakerSettings) http.Handler {
	return &CircuitBreakerMiddleware{
		next:   next,
		config: cfg,
	}
}

func (m *CircuitBreakerMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.state.RLock()
	if m.state.open {
		if time.Since(m.state.lastError) > time.Duration(m.config.Timeout)*time.Second {
			m.state.RUnlock()
			m.state.Lock()
			m.state.open = false
			m.state.failures = 0
			m.state.Unlock()
		} else {
			m.state.RUnlock()
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
	} else {
		m.state.RUnlock()
	}

	// Use a custom response writer to track errors
	crw := &customResponseWriter{ResponseWriter: w}
	m.next.ServeHTTP(crw, r)

	if crw.status >= 500 {
		m.state.Lock()
		m.state.failures++
		if m.state.failures >= m.config.Threshold {
			m.state.open = true
			m.state.lastError = time.Now()
		}
		m.state.Unlock()
	}
}

// CacheMiddleware implements response caching
type CacheMiddleware struct {
	next   http.Handler
	config *config.RouteCacheConfig
	cache  sync.Map
}

func NewCacheMiddleware(next http.Handler, cfg *config.RouteCacheConfig) http.Handler {
	return &CacheMiddleware{
		next:   next,
		config: cfg,
	}
}

func (m *CacheMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Skip caching for non-GET requests
	if r.Method != http.MethodGet {
		m.next.ServeHTTP(w, r)
		return
	}

	// Generate cache key
	key := r.URL.String()
	if m.config.CacheAuthenticated {
		key += r.Header.Get("Authorization")
	}

	// Check cache
	if cached, ok := m.cache.Load(key); ok {
		entry := cached.(*cacheEntry)
		if !entry.expired() {
			for k, v := range entry.headers {
				w.Header().Set(k, v[0]) // Use first value from header slice
			}
			w.Write(entry.body)
			return
		}
		m.cache.Delete(key)
	}

	// Cache miss, serve and store
	crw := &cachingResponseWriter{
		ResponseWriter: w,
		headers:        make(http.Header),
		body:           &strings.Builder{},
	}
	m.next.ServeHTTP(crw, r)

	if crw.status == http.StatusOK {
		m.cache.Store(key, &cacheEntry{
			body:    []byte(crw.body.String()),
			headers: crw.headers,
			expires: time.Now().Add(time.Duration(m.config.TTL) * time.Second),
		})
	}
}

// CompressionMiddleware handles response compression
type CompressionMiddleware struct {
	next http.Handler
}

func NewCompressionMiddleware(next http.Handler) http.Handler {
	return &CompressionMiddleware{next: next}
}

func (m *CompressionMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		m.next.ServeHTTP(w, r)
		return
	}

	gz := gzip.NewWriter(w)
	defer gz.Close()

	w.Header().Set("Content-Encoding", "gzip")
	gzw := gzipResponseWriter{ResponseWriter: w, Writer: gz}
	m.next.ServeHTTP(gzw, r)
}

// CORSMiddleware handles CORS headers
type CORSMiddleware struct {
	next   http.Handler
	config config.CorsConfig
}

func NewCORSMiddleware(next http.Handler, cfg config.CorsConfig) http.Handler {
	return &CORSMiddleware{
		next:   next,
		config: cfg,
	}
}

func (m *CORSMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")

	// Check if origin is allowed
	if !m.config.AllowAllOrigins {
		allowed := false
		for _, allowedOrigin := range m.config.AllowedOrigins {
			if allowedOrigin == origin {
				allowed = true
				break
			}
		}
		if !allowed {
			m.next.ServeHTTP(w, r)
			return
		}
	}

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", origin)
	if m.config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if len(m.config.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.config.AllowedMethods, ", "))
	}
	if len(m.config.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.config.AllowedHeaders, ", "))
	}
	if len(m.config.ExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(m.config.ExposedHeaders, ", "))
	}
	if m.config.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", m.config.MaxAge))
	}

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	m.next.ServeHTTP(w, r)
}

// Helper types

type customResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *customResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *customResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

type cachingResponseWriter struct {
	http.ResponseWriter
	status  int
	headers http.Header
	body    *strings.Builder
}

func (w *cachingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *cachingResponseWriter) Write(b []byte) (int, error) {
	if w.body == nil {
		w.body = &strings.Builder{}
	}
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *cachingResponseWriter) Header() http.Header {
	return w.headers
}

type cacheEntry struct {
	body    []byte
	headers http.Header
	expires time.Time
}

func (e *cacheEntry) expired() bool {
	return time.Now().After(e.expires)
}

type gzipResponseWriter struct {
	http.ResponseWriter
	io.Writer
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}
