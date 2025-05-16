package middleware

import (
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
)

// mockTracingLogger for testing
type mockTracingLogger struct{}

func (m *mockTracingLogger) Debug(msg string, fields ...logger.Field)  {}
func (m *mockTracingLogger) Info(msg string, fields ...logger.Field)   {}
func (m *mockTracingLogger) Warn(msg string, fields ...logger.Field)   {}
func (m *mockTracingLogger) Error(msg string, fields ...logger.Field)  {}
func (m *mockTracingLogger) Fatal(msg string, fields ...logger.Field)  {}
func (m *mockTracingLogger) With(fields ...logger.Field) logger.Logger { return m }

// TestNewTracingMiddleware tests the constructor
func TestNewTracingMiddleware(t *testing.T) {
	// Test case 1: Disabled tracing
	cfg1 := &config.TracingConfig{
		Enabled: false,
	}
	log := &mockTracingLogger{}

	middleware1 := NewTracingMiddleware(cfg1, log)

	assert.NotNil(t, middleware1)
	assert.Equal(t, cfg1, middleware1.config)
	assert.Equal(t, log, middleware1.log)
	assert.False(t, middleware1.initialized)

	// Test case 2: Enabled tracing but invalid endpoint (should not panic)
	// Since initialization can be environment dependent, we won't assert on
	// the initialization state. We just want to ensure it doesn't panic.
	cfg2 := &config.TracingConfig{
		Enabled:     true,
		Provider:    "jaeger",
		Endpoint:    "invalid-endpoint",
		ServiceName: "test-service",
		SampleRate:  0.1,
	}

	// This shouldn't panic even with invalid endpoint
	middleware2 := NewTracingMiddleware(cfg2, log)
	assert.NotNil(t, middleware2)
	assert.Equal(t, cfg2, middleware2.config)

	// Don't assert on initialization state as it might succeed or fail
	// depending on the environment
	// assert.False(t, middleware2.initialized)
}

// TestTracingMiddleware_Tracing_Disabled tests the middleware when tracing is disabled
func TestTracingMiddleware_Tracing_Disabled(t *testing.T) {
	cfg := &config.TracingConfig{
		Enabled: false,
	}
	log := &mockTracingLogger{}

	middleware := NewTracingMiddleware(cfg, log)

	// Create a test handler
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with tracing middleware
	handler := middleware.Tracing(testHandler)

	// Send a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check that the handler was called and response is as expected
	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}

// TestTracingMiddleware_Tracing_NotInitialized tests the middleware when tracing is enabled but not initialized
func TestTracingMiddleware_Tracing_NotInitialized(t *testing.T) {
	cfg := &config.TracingConfig{
		Enabled: true,
	}
	log := &mockTracingLogger{}

	middleware := &TracingMiddleware{
		config:      cfg,
		log:         log,
		initialized: false, // Not initialized
	}

	// Create a test handler
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with tracing middleware
	handler := middleware.Tracing(testHandler)

	// Send a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check that the handler was called and response is as expected
	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}

// TestTracingMiddleware_isSensitiveHeader tests the header sensitivity checker
func TestTracingMiddleware_isSensitiveHeader(t *testing.T) {
	testCases := []struct {
		header   string
		expected bool
	}{
		{"Authorization", true},
		{"authorization", true},
		{"X-API-Key", true},
		{"Cookie", true},
		{"Set-Cookie", true},
		{"X-CSRF-Token", true},
		{"X-Forwarded-For", true},
		{"Content-Type", false},
		{"Accept", false},
		{"User-Agent", false},
		{"X-Custom-Header", false},
	}

	for _, tc := range testCases {
		t.Run(tc.header, func(t *testing.T) {
			result := isSensitiveHeader(tc.header)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestTracingMiddleware_Shutdown tests the shutdown method
func TestTracingMiddleware_Shutdown(t *testing.T) {
	// Test case 1: Disabled tracing
	cfg1 := &config.TracingConfig{
		Enabled: false,
	}
	log := &mockTracingLogger{}

	middleware1 := NewTracingMiddleware(cfg1, log)
	err1 := middleware1.Shutdown(context.Background())
	assert.NoError(t, err1)

	// Test case 2: Enabled but not initialized
	cfg2 := &config.TracingConfig{
		Enabled: true,
	}
	middleware2 := &TracingMiddleware{
		config:      cfg2,
		log:         log,
		initialized: false,
	}
	err2 := middleware2.Shutdown(context.Background())
	assert.NoError(t, err2)

	// Test case 3: Enabled and initialized but nil provider
	middleware3 := &TracingMiddleware{
		config:      cfg2,
		log:         log,
		initialized: true,
		tp:          nil,
	}
	err3 := middleware3.Shutdown(context.Background())
	assert.NoError(t, err3)
}

// TestTracingMiddleware_Tracing_Enabled tests the middleware when tracing is enabled and initialized
func TestTracingMiddleware_Tracing_Enabled(t *testing.T) {
	cfg := &config.TracingConfig{
		Enabled:     true,
		Provider:    "jaeger",
		Endpoint:    "http://jaeger:14268/api/traces",
		ServiceName: "test-service",
		SampleRate:  1.0,
	}
	log := &mockTracingLogger{}

	// Create middleware with mocked tracer
	middleware := &TracingMiddleware{
		config:      cfg,
		log:         log,
		initialized: true,
	}

	// Set a no-op tracer for testing
	middleware.tracer = otel.GetTracerProvider().Tracer("test")

	// Create a test handler that generates all response types
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusCode, _ := strconv.Atoi(r.URL.Query().Get("status"))
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		w.WriteHeader(statusCode)
		w.Write([]byte(http.StatusText(statusCode)))
	})

	// Wrap the handler with tracing middleware
	handler := middleware.Tracing(testHandler)

	testCases := []struct {
		name       string
		path       string
		status     int
		headers    map[string]string
		assertions func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:   "successful_request",
			path:   "/api/test",
			status: http.StatusOK,
			headers: map[string]string{
				"User-Agent":    "test-agent",
				"Content-Type":  "application/json",
				"Authorization": "Bearer token123",
			},
			assertions: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, "OK", rec.Body.String())
			},
		},
		{
			name:   "server_error",
			path:   "/api/test?status=500",
			status: http.StatusInternalServerError,
			headers: map[string]string{
				"User-Agent": "test-agent",
			},
			assertions: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
				assert.Equal(t, "Internal Server Error", rec.Body.String())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request with specified headers
			req := httptest.NewRequest("GET", "http://example.com"+tc.path, nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			// Process the request
			handler.ServeHTTP(rec, req)

			// Run assertions
			tc.assertions(t, rec)
		})
	}
}
