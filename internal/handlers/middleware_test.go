package handlers

import (
	"api-gateway/internal/config"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthMiddleware tests the authentication middleware
func TestAuthMiddleware(t *testing.T) {
	// Setup a mock handler that always succeeds
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	})

	// Create JWT secret for testing
	jwtSecret := "test-secret"

	// Create auth config
	authConfig := config.AuthConfig{
		APIKeyHeader: "X-API-Key",
		JWTHeader:    "Authorization",
		JWTSecret:    jwtSecret,
	}

	// Create the auth middleware
	middleware := NewAuthMiddleware(mockHandler, authConfig)

	testCases := []struct {
		name           string
		headers        map[string]string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "no_auth",
			headers:        map[string]string{},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Authentication required\n",
		},
		{
			name: "valid_api_key",
			headers: map[string]string{
				"X-API-Key": "valid-key", // The middleware.validateAPIKey always returns nil in the code
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "Success",
		},
		{
			name: "valid_jwt",
			headers: map[string]string{
				"Authorization": createValidJWT(t, jwtSecret),
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "Success",
		},
		{
			name: "invalid_jwt",
			headers: map[string]string{
				"Authorization": "Bearer invalid.token.here",
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid token\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			rec := httptest.NewRecorder()
			middleware.ServeHTTP(rec, req)

			assert.Equal(t, tc.expectedStatus, rec.Code)
			assert.Equal(t, tc.expectedBody, rec.Body.String())
		})
	}
}

// Helper function to create a valid JWT for testing
func createValidJWT(t *testing.T, secret string) string {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["sub"] = "1234567890"
	claims["name"] = "Test User"
	claims["exp"] = time.Now().Add(time.Hour * 24).Unix()

	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return "Bearer " + tokenString
}

// TestRateLimitMiddleware tests the rate limiting middleware
func TestRateLimitMiddleware(t *testing.T) {
	// Setup a mock handler that always succeeds
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	})

	// Create rate limit config with a low limit for testing
	rateLimitConfig := &config.RateLimitConfig{
		Requests: 2,    // Allow only 2 requests
		Period:   "1s", // per 1 second
	}

	// Create the rate limit middleware
	middleware := NewRateLimitMiddleware(mockHandler, rateLimitConfig)

	// Test rate limiting
	t.Run("rate_limit_exceeded", func(t *testing.T) {
		clientIP := "192.168.1.1"

		// First request should succeed
		req1 := httptest.NewRequest("GET", "/test", nil)
		req1.Header.Set("X-Real-IP", clientIP)
		rec1 := httptest.NewRecorder()
		middleware.ServeHTTP(rec1, req1)
		assert.Equal(t, http.StatusOK, rec1.Code)

		// Second request should succeed
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.Header.Set("X-Real-IP", clientIP)
		rec2 := httptest.NewRecorder()
		middleware.ServeHTTP(rec2, req2)
		assert.Equal(t, http.StatusOK, rec2.Code)

		// Third request should fail due to rate limit
		req3 := httptest.NewRequest("GET", "/test", nil)
		req3.Header.Set("X-Real-IP", clientIP)
		rec3 := httptest.NewRecorder()
		middleware.ServeHTTP(rec3, req3)
		assert.Equal(t, http.StatusTooManyRequests, rec3.Code)
		assert.Equal(t, "Rate limit exceeded\n", rec3.Body.String())
	})

	// Test with different client IPs
	t.Run("different_clients", func(t *testing.T) {
		// First client
		req1 := httptest.NewRequest("GET", "/test", nil)
		req1.Header.Set("X-Real-IP", "10.0.0.1")
		rec1 := httptest.NewRecorder()
		middleware.ServeHTTP(rec1, req1)
		assert.Equal(t, http.StatusOK, rec1.Code)

		// Second client should be able to make a request
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.Header.Set("X-Real-IP", "10.0.0.2")
		rec2 := httptest.NewRecorder()
		middleware.ServeHTTP(rec2, req2)
		assert.Equal(t, http.StatusOK, rec2.Code)
	})
}

// TestCircuitBreakerMiddleware tests the circuit breaker middleware
func TestCircuitBreakerMiddleware(t *testing.T) {
	// Setup a handler with configurable status code
	statusCode := http.StatusOK
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Write([]byte("Response"))
	})

	// Create circuit breaker config
	cbConfig := &config.CircuitBreakerSettings{
		Threshold: 2, // Open after 2 failures
		Timeout:   1, // 1 second timeout before retrying
		Enabled:   true,
	}

	// Create the circuit breaker middleware
	middleware := NewCircuitBreakerMiddleware(mockHandler, cbConfig)

	t.Run("circuit_opens_after_threshold", func(t *testing.T) {
		// First, trigger failures to open the circuit
		statusCode = http.StatusInternalServerError // Set to error

		// First error
		req1 := httptest.NewRequest("GET", "/test", nil)
		rec1 := httptest.NewRecorder()
		middleware.ServeHTTP(rec1, req1)
		assert.Equal(t, http.StatusInternalServerError, rec1.Code)

		// Second error - should trip the circuit
		req2 := httptest.NewRequest("GET", "/test", nil)
		rec2 := httptest.NewRecorder()
		middleware.ServeHTTP(rec2, req2)
		assert.Equal(t, http.StatusInternalServerError, rec2.Code)

		// Next request should see circuit open
		req3 := httptest.NewRequest("GET", "/test", nil)
		rec3 := httptest.NewRecorder()
		middleware.ServeHTTP(rec3, req3)
		assert.Equal(t, http.StatusServiceUnavailable, rec3.Code)
		assert.Equal(t, "Service unavailable\n", rec3.Body.String())

		// Wait for timeout and circuit should close
		time.Sleep(time.Duration(cbConfig.Timeout+1) * time.Second)

		// Set handler to return success
		statusCode = http.StatusOK

		// Request should go through now
		req4 := httptest.NewRequest("GET", "/test", nil)
		rec4 := httptest.NewRecorder()
		middleware.ServeHTTP(rec4, req4)
		assert.Equal(t, http.StatusOK, rec4.Code)
	})
}

// TestCacheMiddleware tests the caching middleware
func TestCacheMiddleware(t *testing.T) {
	// Setup a handler that tracks calls
	callCount := 0
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Response %d", callCount)))
	})

	// Create cache config
	cacheConfig := &config.RouteCacheConfig{
		Enabled: true,
		TTL:     10,
	}

	// Create the cache middleware
	middleware := NewCacheMiddleware(mockHandler, cacheConfig)

	t.Run("cache_hit", func(t *testing.T) {
		// Reset call count
		callCount = 0

		// First request should cache
		req1 := httptest.NewRequest("GET", "/test", nil)
		rec1 := httptest.NewRecorder()
		middleware.ServeHTTP(rec1, req1)
		assert.Equal(t, http.StatusOK, rec1.Code)
		assert.Equal(t, "Response 1", rec1.Body.String())

		// Second request should use cache
		req2 := httptest.NewRequest("GET", "/test", nil)
		rec2 := httptest.NewRecorder()
		middleware.ServeHTTP(rec2, req2)
		assert.Equal(t, http.StatusOK, rec2.Code)
		assert.Equal(t, "Response 1", rec2.Body.String()) // Should still be first response
		assert.Equal(t, 1, callCount)                     // Handler should only be called once
	})

	t.Run("no_cache_for_post", func(t *testing.T) {
		// Reset call count
		callCount = 0

		// POST request should not be cached
		req1 := httptest.NewRequest("POST", "/test", strings.NewReader("data"))
		rec1 := httptest.NewRecorder()
		middleware.ServeHTTP(rec1, req1)
		assert.Equal(t, 1, callCount)

		// Second POST should not use cache
		req2 := httptest.NewRequest("POST", "/test", strings.NewReader("data"))
		rec2 := httptest.NewRecorder()
		middleware.ServeHTTP(rec2, req2)
		assert.Equal(t, 2, callCount) // Handler should be called twice
	})
}

// TestCompressionMiddleware tests the compression middleware
func TestCompressionMiddleware(t *testing.T) {
	longString := strings.Repeat("test compression content ", 100)

	// Setup a handler that returns a compressible response
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(longString))
	})

	// Create the compression middleware
	middleware := NewCompressionMiddleware(mockHandler)

	t.Run("with_compression", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

		// Check content is compressed
		reader, err := gzip.NewReader(rec.Body)
		require.NoError(t, err)

		decompressed, err := io.ReadAll(reader)
		require.NoError(t, err)

		assert.Equal(t, longString, string(decompressed))
	})

	t.Run("without_compression", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NotEqual(t, "gzip", rec.Header().Get("Content-Encoding"))
		assert.Equal(t, longString, rec.Body.String())
	})
}

// TestCORSMiddleware tests the CORS middleware
func TestCORSMiddleware(t *testing.T) {
	// Setup a mock handler
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	})

	// Create CORS config
	corsConfig := config.CorsConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		ExposedHeaders:   []string{"X-Custom-Header"},
		AllowCredentials: true,
		MaxAge:           3600,
	}

	// Create the CORS middleware
	middleware := NewCORSMiddleware(mockHandler, corsConfig)

	t.Run("preflight_request", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Content-Type")

		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
		assert.Equal(t, "Content-Type, Authorization", rec.Header().Get("Access-Control-Allow-Headers"))
		assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
		assert.Equal(t, "3600", rec.Header().Get("Access-Control-Max-Age"))
	})

	t.Run("normal_request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://example.com")

		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "X-Custom-Header", rec.Header().Get("Access-Control-Expose-Headers"))
		assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
		assert.Equal(t, "Success", rec.Body.String())
	})

	t.Run("disallowed_origin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://malicious.com")

		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	})
}

// Test the custom response writers
func TestCustomResponseWriter(t *testing.T) {
	t.Run("customResponseWriter", func(t *testing.T) {
		w := httptest.NewRecorder()
		crw := &customResponseWriter{ResponseWriter: w}

		crw.WriteHeader(http.StatusCreated)
		crw.Write([]byte("Test"))

		assert.Equal(t, http.StatusCreated, crw.status)
		assert.Equal(t, "Test", w.Body.String())
	})
}

func TestCachingResponseWriter(t *testing.T) {
	t.Run("cachingResponseWriter", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := &strings.Builder{}
		crw := &cachingResponseWriter{
			ResponseWriter: w,
			headers:        make(http.Header),
			body:           body,
		}

		crw.Header().Set("Content-Type", "text/plain")
		crw.WriteHeader(http.StatusOK)
		crw.Write([]byte("Test"))

		assert.Equal(t, http.StatusOK, crw.status)
		assert.Equal(t, "text/plain", crw.headers.Get("Content-Type"))
		assert.Equal(t, "Test", body.String())
	})
}

func TestCacheEntry(t *testing.T) {
	t.Run("expired", func(t *testing.T) {
		pastTime := time.Now().Add(-time.Hour)
		entry := &cacheEntry{
			expires: pastTime,
		}

		assert.True(t, entry.expired())

		futureTime := time.Now().Add(time.Hour)
		entry.expires = futureTime

		assert.False(t, entry.expired())
	})
}

func TestGzipResponseWriter(t *testing.T) {
	t.Run("write", func(t *testing.T) {
		w := httptest.NewRecorder()
		gzipWriter := gzip.NewWriter(w)
		gzw := gzipResponseWriter{
			ResponseWriter: w,
			Writer:         gzipWriter,
		}

		n, err := gzw.Write([]byte("Test"))
		gzipWriter.Close() // Important to flush

		assert.NoError(t, err)
		assert.Equal(t, 4, n)
		assert.NotEmpty(t, w.Body.Bytes()) // Should have gzipped content
	})
}
