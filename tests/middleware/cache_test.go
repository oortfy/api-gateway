package middleware_test

import (
	"api-gateway/internal/config"
	"api-gateway/internal/middleware"
	"api-gateway/pkg/logger"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLogger is a mock implementation of the logger.Logger interface
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Fatal(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

// TestCacheMiddleware_BasicCaching tests that responses are cached correctly
func TestCacheMiddleware_BasicCaching(t *testing.T) {
	// Setup
	mockLogger := new(MockLogger)
	mockLogger.On("Debug", mock.MatchedBy(func(msg string) bool { return true }), mock.Anything).Return()

	cacheConfig := &config.CacheConfig{
		Enabled:     true,
		DefaultTTL:  60,
		MaxTTL:      3600,
		MaxSize:     1000,
		IncludeHost: true,
		VaryHeaders: []string{"Accept-Encoding", "Accept-Language"},
	}

	routeConfig := config.Route{
		Path: "/test",
		Cache: &config.RouteCacheConfig{
			Enabled:            true,
			TTL:                60,
			CacheAuthenticated: false,
		},
	}

	cacheMiddleware := middleware.NewCacheMiddleware(cacheConfig, mockLogger)

	// Define a simple handler that returns a unique timestamp
	handlerCalls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "Response content")
	})

	// Wrap the handler with the cache middleware
	wrappedHandler := cacheMiddleware.Cache(handler, routeConfig)

	// First request - should miss cache
	req1 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	rec1 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec1, req1)

	// Verify first response
	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.Equal(t, "Response content", rec1.Body.String())
	assert.Equal(t, 1, handlerCalls, "Handler should be called on first request")
	assert.Empty(t, rec1.Header().Get("X-Cache"), "X-Cache header should not be set on first request")

	// Second request - should hit cache
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	rec2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec2, req2)

	// Verify second response
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, "Response content", rec2.Body.String())
	assert.Equal(t, 1, handlerCalls, "Handler should not be called on second request")
	assert.Equal(t, "HIT", rec2.Header().Get("X-Cache"), "X-Cache header should indicate a cache hit")

	mockLogger.AssertExpectations(t)
}

// TestCacheMiddleware_ShouldNotCacheNonGetRequests tests that non-GET requests are not cached
func TestCacheMiddleware_ShouldNotCacheNonGetRequests(t *testing.T) {
	// Setup
	mockLogger := new(MockLogger)
	// Don't expect any Debug calls for non-GET requests since they're immediately skipped
	// and no Debug logs are generated

	cacheConfig := &config.CacheConfig{
		Enabled:     true,
		DefaultTTL:  60,
		MaxTTL:      3600,
		MaxSize:     1000,
		IncludeHost: true,
	}

	routeConfig := config.Route{
		Path: "/test",
		Cache: &config.RouteCacheConfig{
			Enabled:            true,
			TTL:                60,
			CacheAuthenticated: false,
		},
	}

	cacheMiddleware := middleware.NewCacheMiddleware(cacheConfig, mockLogger)

	// Define a handler that counts calls
	handlerCalls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "Response for "+r.Method)
	})

	// Wrap the handler with the cache middleware
	wrappedHandler := cacheMiddleware.Cache(handler, routeConfig)

	// POST request - should not be cached
	reqPost := httptest.NewRequest(http.MethodPost, "http://example.com/test", nil)
	recPost := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recPost, reqPost)

	// Another POST request - should invoke handler again
	reqPost2 := httptest.NewRequest(http.MethodPost, "http://example.com/test", nil)
	recPost2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recPost2, reqPost2)

	// Verify POST requests are not cached
	assert.Equal(t, 2, handlerCalls, "Handler should be called for each POST request")
	assert.Empty(t, recPost2.Header().Get("X-Cache"), "X-Cache header should not be set for POST requests")

	// We're not expecting any Debug calls for POST requests
	// so no need to verify mockLogger.AssertExpectations(t)
}

// TestCacheMiddleware_NoCache tests that responses with Cache-Control: no-cache are not cached
func TestCacheMiddleware_NoCache(t *testing.T) {
	// Setup
	mockLogger := new(MockLogger)
	// Don't expect any Debug calls for no-cache requests since they're immediately skipped
	// and no Debug logs are generated

	cacheConfig := &config.CacheConfig{
		Enabled:     true,
		DefaultTTL:  60,
		MaxTTL:      3600,
		MaxSize:     1000,
		IncludeHost: true,
	}

	routeConfig := config.Route{
		Path: "/test",
		Cache: &config.RouteCacheConfig{
			Enabled:            true,
			TTL:                60,
			CacheAuthenticated: false,
		},
	}

	cacheMiddleware := middleware.NewCacheMiddleware(cacheConfig, mockLogger)

	// Define a handler that counts calls
	handlerCalls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "Response content")
	})

	// Wrap the handler with the cache middleware
	wrappedHandler := cacheMiddleware.Cache(handler, routeConfig)

	// First request with Cache-Control: no-cache
	req1 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	req1.Header.Set("Cache-Control", "no-cache")
	rec1 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec1, req1)

	// Second identical request
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	req2.Header.Set("Cache-Control", "no-cache")
	rec2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec2, req2)

	// Verify that both requests called the handler
	assert.Equal(t, 2, handlerCalls, "Handler should be called for each request with no-cache")
	assert.Empty(t, rec2.Header().Get("X-Cache"), "X-Cache header should not be set for no-cache requests")

	// We're not expecting any Debug calls for no-cache requests
	// so no need to verify mockLogger.AssertExpectations(t)
}

// TestCacheMiddleware_VaryHeaders tests that responses vary based on headers
func TestCacheMiddleware_VaryHeaders(t *testing.T) {
	// Setup
	mockLogger := new(MockLogger)
	mockLogger.On("Debug", mock.MatchedBy(func(msg string) bool { return true }), mock.Anything).Return()

	cacheConfig := &config.CacheConfig{
		Enabled:     true,
		DefaultTTL:  60,
		MaxTTL:      3600,
		MaxSize:     1000,
		IncludeHost: true,
		VaryHeaders: []string{"Accept-Encoding", "Accept-Language"},
	}

	routeConfig := config.Route{
		Path: "/test",
		Cache: &config.RouteCacheConfig{
			Enabled:            true,
			TTL:                60,
			CacheAuthenticated: false,
		},
	}

	cacheMiddleware := middleware.NewCacheMiddleware(cacheConfig, mockLogger)

	// Define a handler
	handlerCalls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "Response for "+r.Header.Get("Accept-Encoding"))
	})

	// Wrap the handler with the cache middleware
	wrappedHandler := cacheMiddleware.Cache(handler, routeConfig)

	// First request with gzip encoding
	req1 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	req1.Header.Set("Accept-Encoding", "gzip")
	rec1 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec1, req1)

	// Second request with the same encoding (should be cached)
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	req2.Header.Set("Accept-Encoding", "gzip")
	rec2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec2, req2)

	// Third request with different encoding (should not be cached)
	req3 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	req3.Header.Set("Accept-Encoding", "br")
	rec3 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec3, req3)

	// Verify that requests with different headers have different cache keys
	assert.Equal(t, "Response for gzip", rec1.Body.String())
	assert.Equal(t, "Response for gzip", rec2.Body.String())
	assert.Equal(t, "Response for br", rec3.Body.String())
	assert.Equal(t, 2, handlerCalls, "Handler should be called for first request and request with different header")
	assert.Equal(t, "HIT", rec2.Header().Get("X-Cache"), "Second request should hit cache")
	assert.Empty(t, rec3.Header().Get("X-Cache"), "Request with different header should miss cache")

	mockLogger.AssertExpectations(t)
}

// TestCacheMiddleware_ErrorResponses tests that error responses are not cached
func TestCacheMiddleware_ErrorResponses(t *testing.T) {
	// Setup
	mockLogger := new(MockLogger)
	mockLogger.On("Debug", mock.MatchedBy(func(msg string) bool { return true }), mock.Anything).Return()

	cacheConfig := &config.CacheConfig{
		Enabled:     true,
		DefaultTTL:  60,
		MaxTTL:      3600,
		MaxSize:     1000,
		IncludeHost: true,
	}

	routeConfig := config.Route{
		Path: "/test",
		Cache: &config.RouteCacheConfig{
			Enabled:            true,
			TTL:                60,
			CacheAuthenticated: false,
		},
	}

	cacheMiddleware := middleware.NewCacheMiddleware(cacheConfig, mockLogger)

	// Define a handler that returns different status codes
	handlerCalls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		statusCode, _ := strconv.Atoi(r.URL.Query().Get("status"))
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		w.WriteHeader(statusCode)
		io.WriteString(w, "Response with status "+strconv.Itoa(statusCode))
	})

	// Wrap the handler with the cache middleware
	wrappedHandler := cacheMiddleware.Cache(handler, routeConfig)

	// First request - error response (should not be cached)
	reqError := httptest.NewRequest(http.MethodGet, "http://example.com/test?status=500", nil)
	recError := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recError, reqError)

	// Another identical request - should still invoke handler
	reqError2 := httptest.NewRequest(http.MethodGet, "http://example.com/test?status=500", nil)
	recError2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recError2, reqError2)

	// Verify error responses are not cached
	assert.Equal(t, http.StatusInternalServerError, recError.Code)
	assert.Equal(t, http.StatusInternalServerError, recError2.Code)
	assert.Equal(t, 2, handlerCalls, "Handler should be called for both error requests")
	assert.Empty(t, recError2.Header().Get("X-Cache"), "X-Cache header should not be set for error responses")

	mockLogger.AssertExpectations(t)
}

// TestCacheMiddleware_CacheExpiration tests that cached entries expire
func TestCacheMiddleware_CacheExpiration(t *testing.T) {
	// Setup
	mockLogger := new(MockLogger)
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	// Short TTL for test
	ttl := 1 // 1 second

	cacheConfig := &config.CacheConfig{
		Enabled:     true,
		DefaultTTL:  60,
		MaxTTL:      3600,
		MaxSize:     1000,
		IncludeHost: true,
	}

	routeConfig := config.Route{
		Path: "/test",
		Cache: &config.RouteCacheConfig{
			Enabled:            true,
			TTL:                ttl,
			CacheAuthenticated: false,
		},
	}

	cacheMiddleware := middleware.NewCacheMiddleware(cacheConfig, mockLogger)

	// Define a handler
	handlerCalls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "Response content")
	})

	// Wrap the handler with the cache middleware
	wrappedHandler := cacheMiddleware.Cache(handler, routeConfig)

	// First request - should miss cache
	req1 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	rec1 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec1, req1)
	firstCallCount := handlerCalls // Should be 1

	// Second request - should hit cache
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	rec2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec2, req2)
	secondCallCount := handlerCalls // Should still be 1

	// Wait for cache to expire
	time.Sleep(time.Duration(ttl+1) * time.Second)

	// Third request - should miss cache because entry has expired
	req3 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	rec3 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec3, req3)
	thirdCallCount := handlerCalls // Should be 2

	// Verify that cache is used and expires correctly
	assert.Equal(t, 1, firstCallCount, "Handler should be called once for first request")
	assert.Equal(t, firstCallCount, secondCallCount, "Handler should not be called for second request")
	assert.Equal(t, firstCallCount+1, thirdCallCount, "Handler should be called again after cache expires")
	assert.Equal(t, "HIT", rec2.Header().Get("X-Cache"), "Second request should hit cache")
	assert.Empty(t, rec3.Header().Get("X-Cache"), "X-Cache header should not be set after cache expiration")

	mockLogger.AssertExpectations(t)
}

// TestCacheMiddleware_Disabled tests that the middleware is bypassed when disabled
func TestCacheMiddleware_Disabled(t *testing.T) {
	// Setup with disabled cache
	mockLogger := new(MockLogger)
	// Don't expect any Debug calls when cache is disabled

	cacheConfig := &config.CacheConfig{
		Enabled: false, // Disabled globally
	}

	routeConfig := config.Route{
		Path: "/test",
		Cache: &config.RouteCacheConfig{
			Enabled:            true, // Enabled for route, but global config overrides
			TTL:                60,
			CacheAuthenticated: false,
		},
	}

	// Create middleware with cache disabled
	cacheMiddleware := middleware.NewCacheMiddleware(cacheConfig, mockLogger)

	// Define a handler that counts calls
	handlerCalls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "Response content")
	})

	// Wrap the handler with the cache middleware
	wrappedHandler := cacheMiddleware.Cache(handler, routeConfig)

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	rec1 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec1, req1)

	// Second request
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	rec2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec2, req2)

	// Verify that both requests called the handler because caching is disabled
	assert.Equal(t, 2, handlerCalls, "Handler should be called for each request when caching is disabled")
	assert.Empty(t, rec2.Header().Get("X-Cache"), "X-Cache header should not be set when caching is disabled")

	// We're not expecting any Debug calls when cache is disabled
	// so no need to verify mockLogger.AssertExpectations(t)
}
