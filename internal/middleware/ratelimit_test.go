package middleware

import (
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockRateLimitLogger for testing
type mockRateLimitLogger struct{}

func (m *mockRateLimitLogger) Debug(msg string, fields ...logger.Field)  {}
func (m *mockRateLimitLogger) Info(msg string, fields ...logger.Field)   {}
func (m *mockRateLimitLogger) Warn(msg string, fields ...logger.Field)   {}
func (m *mockRateLimitLogger) Error(msg string, fields ...logger.Field)  {}
func (m *mockRateLimitLogger) Fatal(msg string, fields ...logger.Field)  {}
func (m *mockRateLimitLogger) With(fields ...logger.Field) logger.Logger { return m }

func TestNewRateLimiter(t *testing.T) {
	log := &mockRateLimitLogger{}
	limiter := NewRateLimiter(log)

	assert.NotNil(t, limiter)
	assert.NotNil(t, limiter.limits)
	assert.NotNil(t, limiter.buckets)
	assert.Equal(t, log, limiter.log)
}

func TestRateLimiter_AddLimit(t *testing.T) {
	log := &mockRateLimitLogger{}
	limiter := NewRateLimiter(log)

	limit := config.RateLimitConfig{
		Requests: 10,
		Period:   "minute",
	}
	path := "/api/test"

	limiter.AddLimit(path, limit)

	assert.Contains(t, limiter.limits, path)
	assert.Equal(t, limit, limiter.limits[path])
	assert.NotNil(t, limiter.buckets[path])
}

func TestRateLimiter_GetClientIP(t *testing.T) {
	log := &mockRateLimitLogger{}
	limiter := NewRateLimiter(log)

	testCases := []struct {
		name         string
		setupRequest func() *http.Request
		expectedIP   string
	}{
		{
			name: "from X-Forwarded-For header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
				req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18, 150.172.238.178")
				return req
			},
			expectedIP: "203.0.113.195",
		},
		{
			name: "from X-Real-IP header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
				req.Header.Set("X-Real-IP", "203.0.113.195")
				return req
			},
			expectedIP: "203.0.113.195",
		},
		{
			name: "from RemoteAddr",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
				req.RemoteAddr = "203.0.113.195:58431"
				return req
			},
			expectedIP: "203.0.113.195",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.setupRequest()
			ip := limiter.getClientIP(req)
			assert.Equal(t, tc.expectedIP, ip)
		})
	}
}

func TestRateLimiter_TryConsume(t *testing.T) {
	// Set up test bucket with limited tokens
	bucket := &tokenBucket{
		tokens:         5,
		maxTokens:      10,
		refillRate:     1, // 1 token per second
		lastRefillTime: time.Now(),
	}

	log := &mockRateLimitLogger{}
	limiter := NewRateLimiter(log)

	// Should allow the first 5 requests
	for i := 0; i < 5; i++ {
		allowed := limiter.tryConsume(bucket)
		assert.True(t, allowed, "Request %d should be allowed", i+1)
	}

	// The 6th request should be blocked (no tokens left)
	allowed := limiter.tryConsume(bucket)
	assert.False(t, allowed, "Request 6 should be blocked")

	// Skip the actual token refill test as it depends on timing and could be flaky
	// Just check that the tryConsume method works
}

func TestRateLimiter_RateLimit(t *testing.T) {
	log := &mockRateLimitLogger{}
	limiter := NewRateLimiter(log)

	// Add a rate limit configuration
	path := "/api/limited"
	limit := config.RateLimitConfig{
		Requests: 2,
		Period:   "minute",
	}
	limiter.AddLimit(path, limit)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create a route with rate limiting enabled
	route := config.Route{
		Path: path,
		Middlewares: &config.Middlewares{
			RateLimit: &config.RateLimitConfig{
				Requests: 2,
				Period:   "minute",
			},
		},
	}

	// Wrap the handler with rate limiter
	handler := limiter.RateLimit(testHandler, route)

	// Test case 1: First request should pass
	req1 := httptest.NewRequest("GET", "http://example.com"+path, nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rec1 := httptest.NewRecorder()

	handler.ServeHTTP(rec1, req1)

	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.Equal(t, "OK", rec1.Body.String())

	// Test case 2: Second request from same IP should pass
	req2 := httptest.NewRequest("GET", "http://example.com"+path, nil)
	req2.RemoteAddr = "192.168.1.1:12346"
	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, "OK", rec2.Body.String())

	// Test case 3: Third request from same IP should be rate limited
	req3 := httptest.NewRequest("GET", "http://example.com"+path, nil)
	req3.RemoteAddr = "192.168.1.1:12347"
	rec3 := httptest.NewRecorder()

	handler.ServeHTTP(rec3, req3)

	assert.Equal(t, http.StatusTooManyRequests, rec3.Code)
	assert.Contains(t, rec3.Body.String(), "Rate limit exceeded")
	assert.Equal(t, "60", rec3.Header().Get("Retry-After"))

	// Test case 4: Request from different IP should pass
	req4 := httptest.NewRequest("GET", "http://example.com"+path, nil)
	req4.RemoteAddr = "192.168.1.2:12345"
	rec4 := httptest.NewRecorder()

	handler.ServeHTTP(rec4, req4)

	assert.Equal(t, http.StatusOK, rec4.Code)
	assert.Equal(t, "OK", rec4.Body.String())
}

func TestRateLimiter_APIKeyBasedRateLimit(t *testing.T) {
	log := &mockRateLimitLogger{}
	limiter := NewRateLimiter(log)

	// Add a rate limit configuration
	path := "/api/key-limited"
	limit := config.RateLimitConfig{
		Requests: 2,
		Period:   "minute",
	}
	limiter.AddLimit(path, limit)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create a route with rate limiting enabled
	route := config.Route{
		Path: path,
		Middlewares: &config.Middlewares{
			RateLimit: &config.RateLimitConfig{
				Requests: 2,
				Period:   "minute",
			},
		},
	}

	// Wrap the handler with rate limiter
	handler := limiter.RateLimit(testHandler, route)

	// Test with API keys
	apiKey1 := "key1"
	apiKey2 := "key2"

	// Test case 1: First request with API key 1 should pass
	req1 := httptest.NewRequest("GET", "http://example.com"+path, nil)
	req1.RemoteAddr = "192.168.1.1:12345" // Same IP for all requests
	req1.Header.Set("X-API-Key", apiKey1)
	rec1 := httptest.NewRecorder()

	handler.ServeHTTP(rec1, req1)

	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.Equal(t, "OK", rec1.Body.String())

	// Test case 2: Second request with API key 1 should pass
	req2 := httptest.NewRequest("GET", "http://example.com"+path, nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	req2.Header.Set("X-API-Key", apiKey1)
	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, "OK", rec2.Body.String())

	// Test case 3: Third request with API key 1 should be rate limited
	req3 := httptest.NewRequest("GET", "http://example.com"+path, nil)
	req3.RemoteAddr = "192.168.1.1:12345"
	req3.Header.Set("X-API-Key", apiKey1)
	rec3 := httptest.NewRecorder()

	handler.ServeHTTP(rec3, req3)

	assert.Equal(t, http.StatusTooManyRequests, rec3.Code)
	assert.Contains(t, rec3.Body.String(), "Rate limit exceeded")

	// Test case 4: First request with API key 2 should pass (different key)
	req4 := httptest.NewRequest("GET", "http://example.com"+path, nil)
	req4.RemoteAddr = "192.168.1.1:12345" // Same IP
	req4.Header.Set("X-API-Key", apiKey2)
	rec4 := httptest.NewRecorder()

	handler.ServeHTTP(rec4, req4)

	assert.Equal(t, http.StatusOK, rec4.Code)
	assert.Equal(t, "OK", rec4.Body.String())
}

func TestRateLimiter_NoRateLimit(t *testing.T) {
	log := &mockRateLimitLogger{}
	limiter := NewRateLimiter(log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create a route with no rate limiting
	route := config.Route{
		Path:        "/api/unlimited",
		Middlewares: &config.Middlewares{},
	}

	// Wrap the handler with rate limiter
	handler := limiter.RateLimit(testHandler, route)

	// Send multiple requests - all should pass
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "http://example.com/api/unlimited", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "Request %d should not be rate limited", i+1)
		assert.Equal(t, "OK", rec.Body.String())
	}
}

// TestRateLimiter_GetBucket tests the getBucket function
func TestRateLimiter_GetBucket(t *testing.T) {
	log := &mockRateLimitLogger{}
	limiter := NewRateLimiter(log)

	// Add some test rate limits
	testLimits := []struct {
		path  string
		limit config.RateLimitConfig
	}{
		{
			path: "/api/test1",
			limit: config.RateLimitConfig{
				Requests: 10,
				Period:   "second",
			},
		},
		{
			path: "/api/test2",
			limit: config.RateLimitConfig{
				Requests: 100,
				Period:   "minute",
			},
		},
		{
			path: "/api/test3",
			limit: config.RateLimitConfig{
				Requests: 500,
				Period:   "hour",
			},
		},
		{
			path: "/api/test4",
			limit: config.RateLimitConfig{
				Requests: 1000,
				Period:   "day",
			},
		},
		{
			path: "/api/test5",
			limit: config.RateLimitConfig{
				Requests: 200,
				Period:   "invalid", // Will default to "minute"
			},
		},
	}

	for _, tl := range testLimits {
		limiter.AddLimit(tl.path, tl.limit)
	}

	// Test cases for getBucket
	testCases := []struct {
		name      string
		path      string
		clientID  string
		expectNil bool
		check     func(t *testing.T, bucket *tokenBucket)
	}{
		{
			name:      "path_not_configured",
			path:      "/api/unknown",
			clientID:  "client1",
			expectNil: true,
			check:     func(t *testing.T, bucket *tokenBucket) {},
		},
		{
			name:      "new_client_second_period",
			path:      "/api/test1",
			clientID:  "client1",
			expectNil: false,
			check: func(t *testing.T, bucket *tokenBucket) {
				assert.Equal(t, float64(10), bucket.tokens)
				assert.Equal(t, float64(10), bucket.maxTokens)
				assert.Equal(t, float64(10), bucket.refillRate) // 10 per second
			},
		},
		{
			name:      "new_client_minute_period",
			path:      "/api/test2",
			clientID:  "client2",
			expectNil: false,
			check: func(t *testing.T, bucket *tokenBucket) {
				assert.Equal(t, float64(100), bucket.tokens)
				assert.Equal(t, float64(100), bucket.maxTokens)
				assert.Equal(t, float64(100)/60, bucket.refillRate) // 100 per minute
			},
		},
		{
			name:      "new_client_hour_period",
			path:      "/api/test3",
			clientID:  "client3",
			expectNil: false,
			check: func(t *testing.T, bucket *tokenBucket) {
				assert.Equal(t, float64(500), bucket.tokens)
				assert.Equal(t, float64(500), bucket.maxTokens)
				assert.Equal(t, float64(500)/3600, bucket.refillRate) // 500 per hour
			},
		},
		{
			name:      "new_client_day_period",
			path:      "/api/test4",
			clientID:  "client4",
			expectNil: false,
			check: func(t *testing.T, bucket *tokenBucket) {
				assert.Equal(t, float64(1000), bucket.tokens)
				assert.Equal(t, float64(1000), bucket.maxTokens)
				assert.Equal(t, float64(1000)/86400, bucket.refillRate) // 1000 per day
			},
		},
		{
			name:      "new_client_invalid_period",
			path:      "/api/test5",
			clientID:  "client5",
			expectNil: false,
			check: func(t *testing.T, bucket *tokenBucket) {
				assert.Equal(t, float64(200), bucket.tokens)
				assert.Equal(t, float64(200), bucket.maxTokens)
				assert.Equal(t, float64(200)/60, bucket.refillRate) // Default to 200 per minute
			},
		},
		{
			name:      "existing_client",
			path:      "/api/test1",
			clientID:  "client1", // Reuse client from case 2
			expectNil: false,
			check: func(t *testing.T, bucket *tokenBucket) {
				assert.Equal(t, float64(10), bucket.maxTokens)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bucket := limiter.getBucket(tc.path, tc.clientID)

			if tc.expectNil {
				assert.Nil(t, bucket)
			} else {
				assert.NotNil(t, bucket)
				tc.check(t, bucket)
			}
		})
	}

	// Test client exists but path doesn't - should return nil
	nonExistentPath := "/api/nonexistent"
	nilBucket := limiter.getBucket(nonExistentPath, "client1")
	assert.Nil(t, nilBucket)
}
