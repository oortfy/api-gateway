package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/internal/config"
	"api-gateway/internal/middleware"
	"api-gateway/tests/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCORS_DefaultConfig(t *testing.T) {
	// Create a mock logger
	mockLogger := new(testutils.MockLogger)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Create a default CORS config
	corsConfig := &config.CORSConfig{
		Enabled:          true,
		AllowAllOrigins:  true,
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           86400,
	}

	// Create a new CORS middleware
	cm := middleware.NewCORSMiddleware(corsConfig, mockLogger)

	// Create a test handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Create a test request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("Origin", "http://example.org")
	rec := httptest.NewRecorder()

	// Apply the middleware and handle the request
	handler := cm.CORS(nextHandler)
	handler.ServeHTTP(rec, req)

	// Verify the CORS headers were set correctly
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "test response", rec.Body.String())
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCORS_Disabled(t *testing.T) {
	// Create a mock logger
	mockLogger := new(testutils.MockLogger)

	// Create a disabled CORS config
	corsConfig := &config.CORSConfig{
		Enabled: false,
	}

	// Create a new CORS middleware
	cm := middleware.NewCORSMiddleware(corsConfig, mockLogger)

	// Create a test handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Create a test request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("Origin", "http://example.org")
	rec := httptest.NewRecorder()

	// Apply the middleware and handle the request
	handler := cm.CORS(nextHandler)
	handler.ServeHTTP(rec, req)

	// Verify that no CORS headers were set
	assert.Equal(t, "", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "test response", rec.Body.String())
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCORS_OptionsRequest(t *testing.T) {
	// Create a mock logger
	mockLogger := new(testutils.MockLogger)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Create a CORS config
	corsConfig := &config.CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{"http://example.org"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           86400,
	}

	// Create a new CORS middleware
	cm := middleware.NewCORSMiddleware(corsConfig, mockLogger)

	// Create a test handler (should not be called for OPTIONS requests)
	var handlerCalled bool
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Create a preflight OPTIONS request
	req := httptest.NewRequest("OPTIONS", "http://example.com/test", nil)
	req.Header.Set("Origin", "http://example.org")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	rec := httptest.NewRecorder()

	// Apply the middleware and handle the request
	handler := cm.CORS(nextHandler)
	handler.ServeHTTP(rec, req)

	// Verify preflight response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "http://example.org", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET,POST,PUT,DELETE,OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type,Authorization", rec.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "86400", rec.Header().Get("Access-Control-Max-Age"))

	// Body should be empty and handler should not be called for OPTIONS
	assert.Equal(t, "", rec.Body.String())
	assert.False(t, handlerCalled)
}

func TestCORS_SpecificOrigin(t *testing.T) {
	// Create a mock logger
	mockLogger := new(testutils.MockLogger)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Create a CORS config with specific origins
	corsConfig := &config.CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{"http://allowed.com", "https://also-allowed.com"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           3600,
	}

	// Create a new CORS middleware
	cm := middleware.NewCORSMiddleware(corsConfig, mockLogger)

	// Test with allowed origin
	t.Run("Allowed Origin", func(t *testing.T) {
		// Create a test handler
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		})

		// Create a test request with allowed origin
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		req.Header.Set("Origin", "http://allowed.com")
		rec := httptest.NewRecorder()

		// Apply the middleware and handle the request
		handler := cm.CORS(nextHandler)
		handler.ServeHTTP(rec, req)

		// Verify the CORS headers
		assert.Equal(t, "http://allowed.com", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "test response", rec.Body.String())
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	// Test with disallowed origin
	t.Run("Disallowed Origin", func(t *testing.T) {
		// Create a test handler
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		})

		// Create a test request with disallowed origin
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		req.Header.Set("Origin", "http://disallowed.com")
		rec := httptest.NewRecorder()

		// Apply the middleware and handle the request
		handler := cm.CORS(nextHandler)
		handler.ServeHTTP(rec, req)

		// Origin should not be included in response headers
		assert.Equal(t, "", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "test response", rec.Body.String())
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
