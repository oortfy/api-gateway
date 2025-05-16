package middleware

import (
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockCacheLogger for testing
type mockCacheLogger struct{}

func (m *mockCacheLogger) Debug(msg string, fields ...logger.Field)  {}
func (m *mockCacheLogger) Info(msg string, fields ...logger.Field)   {}
func (m *mockCacheLogger) Warn(msg string, fields ...logger.Field)   {}
func (m *mockCacheLogger) Error(msg string, fields ...logger.Field)  {}
func (m *mockCacheLogger) Fatal(msg string, fields ...logger.Field)  {}
func (m *mockCacheLogger) With(fields ...logger.Field) logger.Logger { return m }

func TestNewCacheMiddleware(t *testing.T) {
	cfg := &config.CacheConfig{
		Enabled:     true,
		DefaultTTL:  60,
		MaxTTL:      3600,
		MaxSize:     1000,
		IncludeHost: true,
		VaryHeaders: []string{"Accept", "Accept-Language"},
	}
	log := &mockCacheLogger{}

	middleware := NewCacheMiddleware(cfg, log)

	assert.NotNil(t, middleware)
	assert.Equal(t, cfg, middleware.config)
	assert.Equal(t, log, middleware.log)
	assert.NotNil(t, middleware.cache)
	assert.Equal(t, 0, len(middleware.cache))
}

func TestCacheMiddleware_ServeFromCache(t *testing.T) {
	cfg := &config.CacheConfig{
		Enabled:    true,
		DefaultTTL: 60,
	}
	log := &mockCacheLogger{}
	middleware := NewCacheMiddleware(cfg, log)

	// Create a mock cache entry
	entry := &CacheEntry{
		StatusCode: http.StatusOK,
		Body:       []byte("Cached response"),
		Headers: http.Header{
			"Content-Type":  []string{"text/plain"},
			"X-Test":        []string{"test-value"},
			"X-Cache-TTL":   []string{"60"},
			"Cache-Control": []string{"public, max-age=60"},
		},
		Expiration: time.Now().Add(60 * time.Second),
	}

	// Create a recorder to capture the response
	rec := httptest.NewRecorder()

	// Serve from cache
	middleware.serveFromCache(rec, entry)

	// Check the response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Cached response", rec.Body.String())
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.Equal(t, "test-value", rec.Header().Get("X-Test"))
	assert.NotEmpty(t, rec.Header().Get("Age"))
	assert.Equal(t, "HIT", rec.Header().Get("X-Cache"))
}

func TestCacheMiddleware_Cache(t *testing.T) {
	cfg := &config.CacheConfig{
		Enabled:    true,
		DefaultTTL: 60,
		MaxTTL:     300,
	}
	log := &mockCacheLogger{}
	middleware := NewCacheMiddleware(cfg, log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello World"))
	})

	// Create a route config with cache enabled
	route := config.Route{
		Path: "/test",
		Middlewares: &config.Middlewares{
			Cache: &config.RouteCacheConfig{
				Enabled: true,
				TTL:     60,
			},
		},
	}

	// Wrap the handler with the cache middleware
	handler := middleware.Cache(testHandler, route)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	rec := httptest.NewRecorder()

	// First request - should cache the response
	handler.ServeHTTP(rec, req)

	// Check the first response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Hello World", rec.Body.String())
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.Equal(t, "MISS", rec.Header().Get("X-Cache"))

	// Second request - should be served from cache
	req = httptest.NewRequest("GET", "http://example.com/test", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check the second response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Hello World", rec.Body.String())
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.Equal(t, "HIT", rec.Header().Get("X-Cache"))
}

func TestCacheMiddleware_PurgeCache(t *testing.T) {
	cfg := &config.CacheConfig{
		Enabled:       true,
		DefaultTTL:    60,
		PurgeEndpoint: "/purge",
	}
	log := &mockCacheLogger{}
	middleware := NewCacheMiddleware(cfg, log)

	// Add an item to the cache
	key := "test-key"
	middleware.cache[key] = &CacheEntry{
		StatusCode: http.StatusOK,
		Body:       []byte("Test cached response"),
		Headers:    http.Header{},
		Expiration: time.Now().Add(60 * time.Second),
	}

	// Create a purge request with the key
	req := httptest.NewRequest("GET", "http://example.com/purge?key="+key, nil)
	rec := httptest.NewRecorder()

	// Call the purge handler
	middleware.PurgeCache(rec, req)

	// Check that the response indicates success
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "purged")

	// Verify the item was removed from cache
	_, exists := middleware.cache[key]
	assert.False(t, exists)

	// Test purging all items
	// Add multiple items to the cache
	middleware.cache["key1"] = &CacheEntry{StatusCode: http.StatusOK, Body: []byte("item1"), Headers: http.Header{}, Expiration: time.Now().Add(60 * time.Second)}
	middleware.cache["key2"] = &CacheEntry{StatusCode: http.StatusOK, Body: []byte("item2"), Headers: http.Header{}, Expiration: time.Now().Add(60 * time.Second)}

	// Create a purge request with no key (purge all)
	req = httptest.NewRequest("GET", "http://example.com/purge", nil)
	rec = httptest.NewRecorder()

	// Call the purge handler
	middleware.PurgeCache(rec, req)

	// Check that the response indicates success
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "all items purged")

	// Verify the cache is empty
	assert.Equal(t, 0, len(middleware.cache))
}

func TestCacheMiddleware_RegisterPurgeEndpoint(t *testing.T) {
	cfg := &config.CacheConfig{
		Enabled:       true,
		DefaultTTL:    60,
		PurgeEndpoint: "/purge",
	}
	log := &mockCacheLogger{}
	middleware := NewCacheMiddleware(cfg, log)

	// Create a basic router
	router := http.NewServeMux()
	router.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Register the purge endpoint
	handler := middleware.RegisterPurgeEndpoint(router)

	// Test a normal request
	req1 := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.Equal(t, "OK", rec1.Body.String())

	// Test the purge endpoint
	req2 := httptest.NewRequest("GET", "http://example.com/purge", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "purged")

	// Test with disabled cache
	cfg.Enabled = false
	disabledMiddleware := NewCacheMiddleware(cfg, log)
	disabledHandler := disabledMiddleware.RegisterPurgeEndpoint(router)

	req3 := httptest.NewRequest("GET", "http://example.com/purge", nil)
	rec3 := httptest.NewRecorder()
	disabledHandler.ServeHTTP(rec3, req3)

	// Should get a 404 as the purge endpoint is not registered when disabled
	assert.Equal(t, http.StatusNotFound, rec3.Code)
}

func TestCacheMiddleware_CacheWithVaryHeader(t *testing.T) {
	cfg := &config.CacheConfig{
		Enabled:     true,
		DefaultTTL:  60,
		VaryHeaders: []string{"Accept", "Accept-Language"},
	}
	log := &mockCacheLogger{}
	middleware := NewCacheMiddleware(cfg, log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if accept == "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message":"Hello JSON"}`))
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<p>Hello HTML</p>"))
		}
	})

	// Create a route config with cache enabled
	route := config.Route{
		Path: "/test",
		Middlewares: &config.Middlewares{
			Cache: &config.RouteCacheConfig{
				Enabled: true,
				TTL:     60,
			},
		},
	}

	// Wrap the handler with the cache middleware
	handler := middleware.Cache(testHandler, route)

	// Request 1: JSON request
	req1 := httptest.NewRequest("GET", "http://example.com/test", nil)
	req1.Header.Set("Accept", "application/json")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.Equal(t, `{"message":"Hello JSON"}`, rec1.Body.String())
	assert.Equal(t, "application/json", rec1.Header().Get("Content-Type"))
	assert.Equal(t, "MISS", rec1.Header().Get("X-Cache"))

	// Request 2: HTML request
	req2 := httptest.NewRequest("GET", "http://example.com/test", nil)
	req2.Header.Set("Accept", "text/html")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, "<p>Hello HTML</p>", rec2.Body.String())
	assert.Equal(t, "text/html", rec2.Header().Get("Content-Type"))
	assert.Equal(t, "MISS", rec2.Header().Get("X-Cache"))

	// Request 3: JSON request again (should be cached)
	req3 := httptest.NewRequest("GET", "http://example.com/test", nil)
	req3.Header.Set("Accept", "application/json")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	assert.Equal(t, http.StatusOK, rec3.Code)
	assert.Equal(t, `{"message":"Hello JSON"}`, rec3.Body.String())
	assert.Equal(t, "application/json", rec3.Header().Get("Content-Type"))
	assert.Equal(t, "HIT", rec3.Header().Get("X-Cache"))
}

func TestCacheMiddleware_CacheControl(t *testing.T) {
	cfg := &config.CacheConfig{
		Enabled:    true,
		DefaultTTL: 60,
		MaxTTL:     300,
	}
	log := &mockCacheLogger{}
	middleware := NewCacheMiddleware(cfg, log)

	// Create a test handler with Cache-Control header
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Cache-Control", "public, max-age=120")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello World"))
	})

	// Create a route config with cache enabled
	route := config.Route{
		Path: "/test",
		Middlewares: &config.Middlewares{
			Cache: &config.RouteCacheConfig{
				Enabled: true,
				TTL:     60, // This should be overridden by Cache-Control
			},
		},
	}

	// Wrap the handler with the cache middleware
	handler := middleware.Cache(testHandler, route)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	rec := httptest.NewRecorder()

	// First request - should cache the response
	handler.ServeHTTP(rec, req)

	// Check the first response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Hello World", rec.Body.String())
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.Equal(t, "MISS", rec.Header().Get("X-Cache"))
	assert.Contains(t, rec.Header().Get("Cache-Control"), "max-age=120")
}

func TestCacheMiddleware_AuthenticatedRequests(t *testing.T) {
	testCases := []struct {
		name                string
		cacheAuthenticated  bool
		expectSecondRequest string
	}{
		{
			name:                "dont_cache_authenticated",
			cacheAuthenticated:  false,
			expectSecondRequest: "MISS",
		},
		{
			name:                "cache_authenticated",
			cacheAuthenticated:  true,
			expectSecondRequest: "HIT",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.CacheConfig{
				Enabled:    true,
				DefaultTTL: 60,
			}
			log := &mockCacheLogger{}
			middleware := NewCacheMiddleware(cfg, log)

			// Create a test handler
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Hello Authenticated User"))
			})

			// Create a route config with cache enabled
			route := config.Route{
				Path: "/test",
				Middlewares: &config.Middlewares{
					Cache: &config.RouteCacheConfig{
						Enabled:            true,
						TTL:                60,
						CacheAuthenticated: tc.cacheAuthenticated,
					},
				},
			}

			// Wrap the handler with the cache middleware
			handler := middleware.Cache(testHandler, route)

			// Create an authenticated request
			req1 := httptest.NewRequest("GET", "http://example.com/test", nil)
			req1.Header.Set("Authorization", "Bearer token123")
			rec1 := httptest.NewRecorder()

			// First request
			handler.ServeHTTP(rec1, req1)

			// Check the first response
			assert.Equal(t, http.StatusOK, rec1.Code)
			assert.Equal(t, "Hello Authenticated User", rec1.Body.String())
			assert.Equal(t, "MISS", rec1.Header().Get("X-Cache"))

			// Second request with same auth
			req2 := httptest.NewRequest("GET", "http://example.com/test", nil)
			req2.Header.Set("Authorization", "Bearer token123")
			rec2 := httptest.NewRecorder()

			// Second request
			handler.ServeHTTP(rec2, req2)

			// Check the second response - should match expectation based on cacheAuthenticated
			assert.Equal(t, http.StatusOK, rec2.Code)
			assert.Equal(t, "Hello Authenticated User", rec2.Body.String())
			assert.Equal(t, tc.expectSecondRequest, rec2.Header().Get("X-Cache"))
		})
	}
}

// TestCacheMiddleware_RemoveFromCache tests removeFromCache function
func TestCacheMiddleware_RemoveFromCache(t *testing.T) {
	cfg := &config.CacheConfig{
		Enabled:    true,
		DefaultTTL: 60,
	}
	log := &mockCacheLogger{}
	middleware := NewCacheMiddleware(cfg, log)

	// Add a few items to the cache
	middleware.cache["key1"] = &CacheEntry{
		StatusCode: http.StatusOK,
		Body:       []byte("Item 1"),
		Headers:    http.Header{},
		Expiration: time.Now().Add(time.Minute),
	}
	middleware.cache["key2"] = &CacheEntry{
		StatusCode: http.StatusOK,
		Body:       []byte("Item 2"),
		Headers:    http.Header{},
		Expiration: time.Now().Add(time.Minute),
	}
	middleware.cache["key3"] = &CacheEntry{
		StatusCode: http.StatusOK,
		Body:       []byte("Item 3"),
		Headers:    http.Header{},
		Expiration: time.Now().Add(time.Minute),
	}

	// Verify initial cache state
	assert.Equal(t, 3, len(middleware.cache))
	assert.NotNil(t, middleware.cache["key1"])
	assert.NotNil(t, middleware.cache["key2"])
	assert.NotNil(t, middleware.cache["key3"])

	// Remove one item
	middleware.removeFromCache("key2")

	// Verify item was removed
	assert.Equal(t, 2, len(middleware.cache))
	assert.NotNil(t, middleware.cache["key1"])
	assert.Nil(t, middleware.cache["key2"]) // This should be nil after removal
	assert.NotNil(t, middleware.cache["key3"])

	// Try to remove a non-existent key (should not cause issues)
	middleware.removeFromCache("non-existent-key")

	// Verify cache state remains unchanged
	assert.Equal(t, 2, len(middleware.cache))
}

// TestCacheMiddleware_GetTTL tests the getTTL function
func TestCacheMiddleware_GetTTL(t *testing.T) {
	cfg := &config.CacheConfig{
		Enabled:    true,
		DefaultTTL: 60,
		MaxTTL:     300, // 5 minutes max
	}
	log := &mockCacheLogger{}
	middleware := NewCacheMiddleware(cfg, log)

	// Define test cases
	testCases := []struct {
		name        string
		headers     http.Header
		routeTTL    int
		expectedTTL time.Duration
		explanation string
	}{
		{
			name:        "default_ttl",
			headers:     http.Header{},
			routeTTL:    120,
			expectedTTL: 120 * time.Second,
			explanation: "Should use route TTL when no cache control headers",
		},
		{
			name: "cache_control_max_age",
			headers: http.Header{
				"Cache-Control": []string{"public, max-age=180"},
			},
			routeTTL:    60,
			expectedTTL: 180 * time.Second,
			explanation: "Should use max-age from Cache-Control header",
		},
		{
			name: "cache_control_max_age_exceeds_max_ttl",
			headers: http.Header{
				"Cache-Control": []string{"public, max-age=600"},
			},
			routeTTL:    60,
			expectedTTL: 300 * time.Second, // Limited by MaxTTL
			explanation: "Should limit TTL to MaxTTL when Cache-Control max-age exceeds it",
		},
		{
			name: "cache_control_no_cache",
			headers: http.Header{
				"Cache-Control": []string{"no-cache, no-store"},
			},
			routeTTL:    60,
			expectedTTL: 60 * time.Second,
			explanation: "Should use route TTL when Cache-Control is no-cache",
		},
		{
			name: "cache_control_invalid_max_age",
			headers: http.Header{
				"Cache-Control": []string{"public, max-age=invalid"},
			},
			routeTTL:    60,
			expectedTTL: 60 * time.Second,
			explanation: "Should use route TTL when Cache-Control max-age is invalid",
		},
		{
			name: "expires_header",
			headers: http.Header{
				"Expires": []string{time.Now().Add(240 * time.Second).Format(time.RFC1123)},
			},
			routeTTL: 60,
			// We can't assert exact TTL because it depends on current time
			// but it should be close to 240 seconds
			explanation: "Should use TTL derived from Expires header",
		},
		{
			name: "zero_ttl",
			headers: http.Header{
				"Cache-Control": []string{"max-age=0"},
			},
			routeTTL:    60,
			expectedTTL: 0,
			explanation: "Should return zero TTL when max-age is 0 (don't cache)",
		},
		{
			name: "negative_ttl",
			headers: http.Header{
				"Cache-Control": []string{"max-age=-10"},
			},
			routeTTL:    60,
			expectedTTL: 0,
			explanation: "Should return zero TTL when max-age is negative (don't cache)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			route := config.Route{
				Middlewares: &config.Middlewares{
					Cache: &config.RouteCacheConfig{
						Enabled: true,
						TTL:     tc.routeTTL,
					},
				},
			}

			req := httptest.NewRequest("GET", "http://example.com/test", nil)
			ttl := middleware.getTTL(req, tc.headers, route)

			if tc.name == "expires_header" {
				// For expires header, we can only check it's in a reasonable range
				assert.Greater(t, ttl, time.Duration(0))
				assert.LessOrEqual(t, ttl, 300*time.Second) // Should be limited by MaxTTL
			} else {
				assert.Equal(t, tc.expectedTTL, ttl, tc.explanation)
			}
		})
	}
}

// TestCacheMiddleware_ShouldCache tests the shouldCache function
func TestCacheMiddleware_ShouldCache(t *testing.T) {
	cfg := &config.CacheConfig{
		Enabled:    true,
		DefaultTTL: 60,
	}
	log := &mockCacheLogger{}
	middleware := NewCacheMiddleware(cfg, log)

	testCases := []struct {
		name     string
		method   string
		setup    func(r *http.Request, route *config.Route)
		expected bool
	}{
		{
			name:   "cache_disabled_globally",
			method: "GET",
			setup: func(r *http.Request, route *config.Route) {
				middleware.config.Enabled = false
				route.Middlewares.Cache.Enabled = true
			},
			expected: false,
		},
		{
			name:   "cache_disabled_for_route",
			method: "GET",
			setup: func(r *http.Request, route *config.Route) {
				middleware.config.Enabled = true
				route.Middlewares.Cache.Enabled = false
			},
			expected: false,
		},
		{
			name:   "non_get_request",
			method: "POST",
			setup: func(r *http.Request, route *config.Route) {
				middleware.config.Enabled = true
				route.Middlewares.Cache.Enabled = true
			},
			expected: false,
		},
		{
			name:   "cache_control_no_cache",
			method: "GET",
			setup: func(r *http.Request, route *config.Route) {
				middleware.config.Enabled = true
				route.Middlewares.Cache.Enabled = true
				r.Header.Set("Cache-Control", "no-cache")
			},
			expected: false,
		},
		{
			name:   "cache_control_no_store",
			method: "GET",
			setup: func(r *http.Request, route *config.Route) {
				middleware.config.Enabled = true
				route.Middlewares.Cache.Enabled = true
				r.Header.Set("Cache-Control", "no-store")
			},
			expected: false,
		},
		{
			name:   "auth_request_no_cache_auth",
			method: "GET",
			setup: func(r *http.Request, route *config.Route) {
				middleware.config.Enabled = true
				route.Middlewares.Cache.Enabled = true
				route.Middlewares.Cache.CacheAuthenticated = false
				r.Header.Set("Authorization", "Bearer token123")
			},
			expected: false,
		},
		{
			name:   "auth_request_with_cache_auth",
			method: "GET",
			setup: func(r *http.Request, route *config.Route) {
				middleware.config.Enabled = true
				route.Middlewares.Cache.Enabled = true
				route.Middlewares.Cache.CacheAuthenticated = true
				r.Header.Set("Authorization", "Bearer token123")
			},
			expected: true,
		},
		{
			name:   "api_key_auth_no_cache_auth",
			method: "GET",
			setup: func(r *http.Request, route *config.Route) {
				middleware.config.Enabled = true
				route.Middlewares.Cache.Enabled = true
				route.Middlewares.Cache.CacheAuthenticated = false
				r.Header.Set("x-api-key", "apikey123")
			},
			expected: false,
		},
		{
			name:   "normal_get_request",
			method: "GET",
			setup: func(r *http.Request, route *config.Route) {
				middleware.config.Enabled = true
				route.Middlewares.Cache.Enabled = true
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://example.com/test", nil)
			route := config.Route{
				Path: "/test",
				Middlewares: &config.Middlewares{
					Cache: &config.RouteCacheConfig{
						Enabled: true,
						TTL:     60,
					},
				},
			}

			tc.setup(req, &route)
			result := middleware.shouldCache(req, route)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestCacheMiddleware_StoreInCache_Eviction tests the storeInCache function with cache eviction
func TestCacheMiddleware_StoreInCache_Eviction(t *testing.T) {
	cfg := &config.CacheConfig{
		Enabled:    true,
		DefaultTTL: 60,
		MaxTTL:     300,
		MaxSize:    3, // Small max size to trigger eviction
	}
	log := &mockCacheLogger{}
	middleware := NewCacheMiddleware(cfg, log)

	// Add several entries to fill the cache
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key%d", i)
		body := []byte(fmt.Sprintf("content%d", i))
		headers := make(http.Header)
		headers.Set("Content-Type", "text/plain")

		middleware.storeInCache(key, http.StatusOK, body, headers, 60*time.Second)
	}

	// Cache should have evicted oldest entries
	assert.LessOrEqual(t, len(middleware.cache), cfg.MaxSize)

	// The last entries should still be in the cache
	key4 := "key4"
	entry, exists := middleware.cache[key4]
	assert.True(t, exists)
	assert.NotNil(t, entry)
	assert.Equal(t, http.StatusOK, entry.StatusCode)
	assert.Equal(t, []byte("content4"), entry.Body)
}

// TestCacheMiddleware_PurgeCache_WithPathPattern tests PurgeCache with a specific path pattern
func TestCacheMiddleware_PurgeCache_WithPathPattern(t *testing.T) {
	cfg := &config.CacheConfig{
		Enabled:       true,
		DefaultTTL:    60,
		PurgeEndpoint: "/purge",
	}
	log := &mockCacheLogger{}
	middleware := NewCacheMiddleware(cfg, log)

	// Add items to the cache with different patterns
	patterns := []string{
		"article/123",
		"article/456",
		"product/789",
		"category/abc",
	}

	for _, pattern := range patterns {
		middleware.cache[pattern] = &CacheEntry{
			StatusCode: http.StatusOK,
			Body:       []byte("Content for " + pattern),
			Headers:    http.Header{},
			Expiration: time.Now().Add(60 * time.Second),
		}
	}

	// Verify initial cache state
	assert.Equal(t, 4, len(middleware.cache))

	// Create a purge request for specific pattern
	req := httptest.NewRequest("POST", "http://example.com/purge?path=article", nil)
	rec := httptest.NewRecorder()

	// Call the purge handler
	middleware.PurgeCache(rec, req)

	// Check response
	assert.Equal(t, http.StatusOK, rec.Code)

	// Should have purged only article paths
	assert.Equal(t, 2, len(middleware.cache))

	// Verify article entries are gone
	_, exists1 := middleware.cache["article/123"]
	_, exists2 := middleware.cache["article/456"]
	_, exists3 := middleware.cache["product/789"]
	_, exists4 := middleware.cache["category/abc"]

	assert.False(t, exists1, "article/123 should be purged")
	assert.False(t, exists2, "article/456 should be purged")
	assert.True(t, exists3, "product/789 should remain")
	assert.True(t, exists4, "category/abc should remain")
}

// TestCacheMiddleware_PurgeCache_MethodNotAllowed tests PurgeCache with invalid HTTP method
func TestCacheMiddleware_PurgeCache_MethodNotAllowed(t *testing.T) {
	cfg := &config.CacheConfig{
		Enabled:       true,
		DefaultTTL:    60,
		PurgeEndpoint: "/purge",
	}
	log := &mockCacheLogger{}
	middleware := NewCacheMiddleware(cfg, log)

	// Create a request with invalid method
	req := httptest.NewRequest("PUT", "http://example.com/purge", nil)
	rec := httptest.NewRecorder()

	// Call the purge handler
	middleware.PurgeCache(rec, req)

	// Should return method not allowed
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Contains(t, rec.Body.String(), "Method not allowed")
}
