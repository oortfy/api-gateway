package middleware

import (
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockTransformLogger for testing
type mockTransformLogger struct{}

func (m *mockTransformLogger) Debug(msg string, fields ...logger.Field)  {}
func (m *mockTransformLogger) Info(msg string, fields ...logger.Field)   {}
func (m *mockTransformLogger) Warn(msg string, fields ...logger.Field)   {}
func (m *mockTransformLogger) Error(msg string, fields ...logger.Field)  {}
func (m *mockTransformLogger) Fatal(msg string, fields ...logger.Field)  {}
func (m *mockTransformLogger) With(fields ...logger.Field) logger.Logger { return m }

func TestNewHeaderTransformer(t *testing.T) {
	log := &mockTransformLogger{}

	transformer := NewHeaderTransformer(log)

	assert.NotNil(t, transformer)
	assert.Equal(t, log, transformer.log)
}

func TestHeaderTransformer_Transform_NilConfig(t *testing.T) {
	log := &mockTransformLogger{}
	transformer := NewHeaderTransformer(log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Original", "value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with nil transform configuration
	handler := transformer.Transform(testHandler, nil)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check if original behavior is preserved
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
	assert.Equal(t, "value", rec.Header().Get("X-Original"))
}

func TestHeaderTransformer_TransformRequestHeaders(t *testing.T) {
	log := &mockTransformLogger{}
	transformer := NewHeaderTransformer(log)

	// Create a test handler that echoes a request header
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the transformed header back in the response
		w.Header().Set("Echo-Header", r.Header.Get("X-Transformed"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create header transform configuration for request
	transform := &config.HeaderTransform{
		Request: map[string]string{
			"X-Transformed": "transformed-value",
		},
	}

	// Wrap the handler with the transformer
	handler := transformer.Transform(testHandler, transform)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check if the request header was transformed
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "transformed-value", rec.Header().Get("Echo-Header"))
}

func TestHeaderTransformer_TransformResponseHeaders(t *testing.T) {
	log := &mockTransformLogger{}
	transformer := NewHeaderTransformer(log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set an original header that will be transformed
		w.Header().Set("X-Original", "original-value")
		// Set another header that will be preserved
		w.Header().Set("X-Preserved", "preserved-value")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create header transform configuration for response
	transform := &config.HeaderTransform{
		Response: map[string]string{
			"X-Original":  "new-value",   // Transform existing header
			"X-Added":     "added-value", // Add new header
			"X-To-Remove": "",            // Empty value should remove header
		},
	}

	// Wrap the handler with the transformer
	handler := transformer.Transform(testHandler, transform)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check response headers
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())

	// Check if header was transformed
	assert.Equal(t, "new-value", rec.Header().Get("X-Original"))

	// Check if new header was added
	assert.Equal(t, "added-value", rec.Header().Get("X-Added"))

	// Check if original header was preserved
	assert.Equal(t, "preserved-value", rec.Header().Get("X-Preserved"))

	// Check if header was removed (empty value)
	assert.Equal(t, "", rec.Header().Get("X-To-Remove"))
}

func TestHeaderTransformer_RemoveHeaders(t *testing.T) {
	log := &mockTransformLogger{}
	transformer := NewHeaderTransformer(log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers that will be removed
		w.Header().Set("X-Remove-1", "value1")
		w.Header().Set("X-Remove-2", "value2")
		// Set a header that will be kept
		w.Header().Set("X-Keep", "value-keep")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create header transform configuration
	transform := &config.HeaderTransform{
		Remove: []string{"X-Remove-1", "X-Remove-2"},
	}

	// Wrap the handler with the transformer
	handler := transformer.Transform(testHandler, transform)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check response
	assert.Equal(t, http.StatusOK, rec.Code)

	// Check if headers were removed
	assert.Equal(t, "", rec.Header().Get("X-Remove-1"))
	assert.Equal(t, "", rec.Header().Get("X-Remove-2"))

	// Check if non-removed header was preserved
	assert.Equal(t, "value-keep", rec.Header().Get("X-Keep"))
}

func TestHeaderTransformer_TransformWithoutExplicitWriteHeader(t *testing.T) {
	log := &mockTransformLogger{}
	transformer := NewHeaderTransformer(log)

	// Create a test handler that doesn't explicitly call WriteHeader
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set a header to be transformed
		w.Header().Set("X-Original", "original-value")
		// Just write the body, relying on implicit 200 OK
		w.Write([]byte("OK"))
	})

	// Create header transform configuration
	transform := &config.HeaderTransform{
		Response: map[string]string{
			"X-Original": "new-value",
			"X-Added":    "added-value",
		},
	}

	// Wrap the handler with the transformer
	handler := transformer.Transform(testHandler, transform)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())

	// Check if headers were transformed correctly
	assert.Equal(t, "new-value", rec.Header().Get("X-Original"))
	assert.Equal(t, "added-value", rec.Header().Get("X-Added"))
}

func TestHeaderTransformer_WriteHeaderOnlyOnce(t *testing.T) {
	log := &mockTransformLogger{}
	transformer := NewHeaderTransformer(log)

	// Create a test handler that calls WriteHeader multiple times
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers to be transformed
		w.Header().Set("X-Original", "original-value")

		// First call to WriteHeader - should set status to 201
		w.WriteHeader(http.StatusCreated)

		// Second call to WriteHeader - should be ignored
		w.WriteHeader(http.StatusBadRequest)

		w.Write([]byte("Created"))
	})

	// Create header transform configuration
	transform := &config.HeaderTransform{
		Response: map[string]string{
			"X-Original": "new-value",
		},
	}

	// Wrap the handler with the transformer
	handler := transformer.Transform(testHandler, transform)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check response - status should be 201, not 400
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "Created", rec.Body.String())
	assert.Equal(t, "new-value", rec.Header().Get("X-Original"))
}
