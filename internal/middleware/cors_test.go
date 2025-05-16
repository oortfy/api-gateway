package middleware

import (
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockCORSLogger for testing
type mockCORSLogger struct{}

func (m *mockCORSLogger) Debug(msg string, fields ...logger.Field)  {}
func (m *mockCORSLogger) Info(msg string, fields ...logger.Field)   {}
func (m *mockCORSLogger) Warn(msg string, fields ...logger.Field)   {}
func (m *mockCORSLogger) Error(msg string, fields ...logger.Field)  {}
func (m *mockCORSLogger) Fatal(msg string, fields ...logger.Field)  {}
func (m *mockCORSLogger) With(fields ...logger.Field) logger.Logger { return m }

func TestNewCORSMiddleware(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:         true,
		AllowAllOrigins: true,
	}
	log := &mockCORSLogger{}

	middleware := NewCORSMiddleware(cfg, log)

	assert.NotNil(t, middleware)
	assert.Equal(t, cfg, middleware.config)
	assert.Equal(t, log, middleware.log)
}

func TestCORSMiddleware_CORS_Disabled(t *testing.T) {
	// Setup
	cfg := &config.CORSConfig{
		Enabled: false,
	}
	log := &mockCORSLogger{}

	middleware := NewCORSMiddleware(cfg, log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with CORS middleware
	handler := middleware.CORS(testHandler)

	// Create a request with origin header
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	req.Header.Set("Origin", "http://allowed-origin.com")
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check if CORS headers are NOT set (as CORS is disabled)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_CORS_AllowAllOrigins(t *testing.T) {
	// Setup
	cfg := &config.CORSConfig{
		Enabled:         true,
		AllowAllOrigins: true,
		AllowedMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:  []string{"Content-Type", "Authorization"},
		ExposedHeaders:  []string{"X-Custom-Header"},
		MaxAge:          86400,
	}
	log := &mockCORSLogger{}

	middleware := NewCORSMiddleware(cfg, log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with CORS middleware
	handler := middleware.CORS(testHandler)

	// Create a request with origin header
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check if CORS headers are set correctly
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_CORS_SpecificOrigin(t *testing.T) {
	// Setup
	allowedOrigin := "http://allowed-origin.com"
	cfg := &config.CORSConfig{
		Enabled:         true,
		AllowAllOrigins: false,
		AllowedOrigins:  []string{allowedOrigin},
		AllowedMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:  []string{"Content-Type", "Authorization"},
		ExposedHeaders:  []string{"X-Custom-Header"},
		MaxAge:          86400,
	}
	log := &mockCORSLogger{}

	middleware := NewCORSMiddleware(cfg, log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with CORS middleware
	handler := middleware.CORS(testHandler)

	// Test case 1: Request with allowed origin
	req1 := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	req1.Header.Set("Origin", allowedOrigin)
	rec1 := httptest.NewRecorder()

	handler.ServeHTTP(rec1, req1)

	// Check if CORS headers are set correctly
	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.Equal(t, allowedOrigin, rec1.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", rec1.Header().Get("Vary"))
	assert.Equal(t, "X-Custom-Header", rec1.Header().Get("Access-Control-Expose-Headers"))

	// Test case 2: Request with disallowed origin
	req2 := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	req2.Header.Set("Origin", "http://disallowed-origin.com")
	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)

	// Check that CORS headers are not set
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, "", rec2.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_CORS_Preflight(t *testing.T) {
	// Setup
	allowedOrigin := "http://allowed-origin.com"
	cfg := &config.CORSConfig{
		Enabled:          true,
		AllowAllOrigins:  false,
		AllowedOrigins:   []string{allowedOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		ExposedHeaders:   []string{"X-Custom-Header"},
		AllowCredentials: true,
		MaxAge:           86400,
	}
	log := &mockCORSLogger{}

	middleware := NewCORSMiddleware(cfg, log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with CORS middleware
	handler := middleware.CORS(testHandler)

	// Create a preflight OPTIONS request
	req := httptest.NewRequest("OPTIONS", "http://example.com/api/test", nil)
	req.Header.Set("Origin", allowedOrigin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type,Authorization")
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check if preflight response is correct
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, allowedOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "86400", rec.Header().Get("Access-Control-Max-Age"))

	// Check if all allowed methods are included
	allowedMethods := rec.Header().Get("Access-Control-Allow-Methods")
	for _, method := range cfg.AllowedMethods {
		assert.Contains(t, allowedMethods, method)
	}

	// Check if all allowed headers are included
	allowedHeaders := rec.Header().Get("Access-Control-Allow-Headers")
	for _, header := range cfg.AllowedHeaders {
		assert.Contains(t, allowedHeaders, header)
	}
}

func TestCORSMiddleware_CORS_Wildcard(t *testing.T) {
	// Setup
	cfg := &config.CORSConfig{
		Enabled:         true,
		AllowAllOrigins: false,
		AllowedOrigins:  []string{"*"},
		AllowedMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:  []string{"Content-Type", "Authorization"},
		MaxAge:          86400,
	}
	log := &mockCORSLogger{}

	middleware := NewCORSMiddleware(cfg, log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with CORS middleware
	handler := middleware.CORS(testHandler)

	// Create a request with any origin
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check if CORS headers use wildcard
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_CORS_WithCredentials(t *testing.T) {
	// Setup
	allowedOrigin := "http://allowed-origin.com"
	cfg := &config.CORSConfig{
		Enabled:          true,
		AllowAllOrigins:  false,
		AllowedOrigins:   []string{allowedOrigin},
		AllowCredentials: true,
	}
	log := &mockCORSLogger{}

	middleware := NewCORSMiddleware(cfg, log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with CORS middleware
	handler := middleware.CORS(testHandler)

	// Create a request with allowed origin
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	req.Header.Set("Origin", allowedOrigin)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check if credentials are allowed
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, allowedOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORSMiddleware_CORS_NoOrigin(t *testing.T) {
	// Setup
	cfg := &config.CORSConfig{
		Enabled:         true,
		AllowAllOrigins: true,
	}
	log := &mockCORSLogger{}

	middleware := NewCORSMiddleware(cfg, log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with CORS middleware
	handler := middleware.CORS(testHandler)

	// Create a request with no origin header (not a CORS request)
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check that no CORS headers are set
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "OK", rec.Body.String())
}

func TestCORSResponseWriter_WriteHeader(t *testing.T) {
	// Setup
	cfg := &config.CORSConfig{
		Enabled:         true,
		AllowAllOrigins: true,
	}
	log := &mockCORSLogger{}

	// Create a test response recorder
	rec := httptest.NewRecorder()

	// Create CORS response writer
	writer := &corsResponseWriter{
		ResponseWriter: rec,
		config:         cfg,
		origin:         "http://example.com",
		log:            log,
	}

	// Call WriteHeader
	writer.WriteHeader(http.StatusCreated)

	// Check if headers were set correctly
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSResponseWriter_Write(t *testing.T) {
	// Setup
	cfg := &config.CORSConfig{
		Enabled:         true,
		AllowAllOrigins: true,
	}
	log := &mockCORSLogger{}

	// Create a test response recorder
	rec := httptest.NewRecorder()

	// Create CORS response writer
	writer := &corsResponseWriter{
		ResponseWriter: rec,
		config:         cfg,
		origin:         "http://example.com",
		log:            log,
	}

	// Call Write
	content := []byte("Test content")
	n, err := writer.Write(content)

	// Check if write was successful
	assert.NoError(t, err)
	assert.Equal(t, len(content), n)
	assert.Equal(t, "Test content", rec.Body.String())

	// Check if headers were set correctly
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}
