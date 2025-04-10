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

func TestHeaderTransformer_NilConfig(t *testing.T) {
	// Setup
	mockLogger := &testutils.MockLogger{}
	mockLogger.On("Error", mock.Anything, mock.Anything).Maybe()

	transformer := middleware.NewHeaderTransformer(mockLogger)

	// Create a test handler that we'll wrap
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	// Create the middleware with nil config
	middlewareHandler := transformer.Transform(testHandler, nil)

	// Create a test request and response recorder
	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Original-Header", "original-value")
	res := httptest.NewRecorder()

	// Execute
	middlewareHandler.ServeHTTP(res, req)

	// Assert
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "test", res.Body.String())
	assert.Equal(t, "original-value", req.Header.Get("Original-Header"))

	mockLogger.AssertExpectations(t)
}

func TestHeaderTransformer_RequestTransform(t *testing.T) {
	// Setup
	mockLogger := &testutils.MockLogger{}
	mockLogger.On("Error", mock.Anything, mock.Anything).Maybe()

	transformer := middleware.NewHeaderTransformer(mockLogger)

	// Create a test handler that verifies headers
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assert the headers were transformed as expected
		assert.Equal(t, "new-value", r.Header.Get("Test-Header"))
		assert.Equal(t, "original-value", r.Header.Get("Original-Header"))
		assert.Equal(t, "added-value", r.Header.Get("Added-Header"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	// Create header transform config
	transform := &config.HeaderTransform{
		Request: map[string]string{
			"Test-Header":  "new-value",
			"Added-Header": "added-value",
		},
		Response: map[string]string{},
		Remove:   []string{},
	}

	// Create the middleware with config
	middlewareHandler := transformer.Transform(testHandler, transform)

	// Create a test request and response recorder
	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Original-Header", "original-value")
	req.Header.Set("Test-Header", "original-test-value")
	res := httptest.NewRecorder()

	// Execute
	middlewareHandler.ServeHTTP(res, req)

	// Assert
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "test", res.Body.String())

	mockLogger.AssertExpectations(t)
}

func TestHeaderTransformer_ResponseTransform(t *testing.T) {
	// Setup
	mockLogger := &testutils.MockLogger{}
	mockLogger.On("Error", mock.Anything, mock.Anything).Maybe()

	transformer := middleware.NewHeaderTransformer(mockLogger)

	// Create a test handler that sets some response headers
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Original-Response", "original-value")
		w.Header().Set("To-Be-Changed", "original-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	// Create header transform config
	transform := &config.HeaderTransform{
		Request: map[string]string{},
		Response: map[string]string{
			"To-Be-Changed":      "new-value",
			"Added-Response":     "added-value",
			"Empty-Value-Header": "", // Should remove this header
		},
		Remove: []string{},
	}

	// Create the middleware with config
	middlewareHandler := transformer.Transform(testHandler, transform)

	// Create a test request and response recorder
	req := httptest.NewRequest("GET", "http://example.com", nil)
	res := httptest.NewRecorder()

	// Execute
	middlewareHandler.ServeHTTP(res, req)

	// Assert
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "test", res.Body.String())
	assert.Equal(t, "original-value", res.Header().Get("Original-Response"))
	assert.Equal(t, "new-value", res.Header().Get("To-Be-Changed"))
	assert.Equal(t, "added-value", res.Header().Get("Added-Response"))
	assert.Equal(t, "", res.Header().Get("Empty-Value-Header"))

	mockLogger.AssertExpectations(t)
}

func TestHeaderTransformer_RemoveHeaders(t *testing.T) {
	// Setup
	mockLogger := &testutils.MockLogger{}
	mockLogger.On("Error", mock.Anything, mock.Anything).Maybe()

	transformer := middleware.NewHeaderTransformer(mockLogger)

	// Create a test handler that sets some response headers
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("To-Be-Removed", "some-value")
		w.Header().Set("Keep-This", "keep-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	// Create header transform config
	transform := &config.HeaderTransform{
		Request:  map[string]string{},
		Response: map[string]string{},
		Remove:   []string{"To-Be-Removed", "Non-Existent-Header"},
	}

	// Create the middleware with config
	middlewareHandler := transformer.Transform(testHandler, transform)

	// Create a test request and response recorder
	req := httptest.NewRequest("GET", "http://example.com", nil)
	res := httptest.NewRecorder()

	// Execute
	middlewareHandler.ServeHTTP(res, req)

	// Assert
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "test", res.Body.String())
	assert.Equal(t, "", res.Header().Get("To-Be-Removed"))
	assert.Equal(t, "keep-value", res.Header().Get("Keep-This"))

	mockLogger.AssertExpectations(t)
}

func TestHeaderTransformer_CompleteTransformation(t *testing.T) {
	// Setup
	mockLogger := &testutils.MockLogger{}
	mockLogger.On("Error", mock.Anything, mock.Anything).Maybe()

	transformer := middleware.NewHeaderTransformer(mockLogger)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request headers
		assert.Equal(t, "new-value", r.Header.Get("Request-Header"))

		// Set response headers
		w.Header().Set("Original-Response", "original-value")
		w.Header().Set("To-Be-Changed", "original-value")
		w.Header().Set("To-Be-Removed", "remove-me")

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("success"))
	})

	// Create header transform config with all types of transformations
	transform := &config.HeaderTransform{
		Request: map[string]string{
			"Request-Header": "new-value",
		},
		Response: map[string]string{
			"To-Be-Changed":  "changed-value",
			"Added-Response": "added-value",
		},
		Remove: []string{"To-Be-Removed"},
	}

	// Create the middleware with config
	middlewareHandler := transformer.Transform(testHandler, transform)

	// Create a test request and response recorder
	req := httptest.NewRequest("GET", "http://example.com", nil)
	res := httptest.NewRecorder()

	// Execute
	middlewareHandler.ServeHTTP(res, req)

	// Assert
	assert.Equal(t, http.StatusCreated, res.Code)
	assert.Equal(t, "success", res.Body.String())
	assert.Equal(t, "original-value", res.Header().Get("Original-Response"))
	assert.Equal(t, "changed-value", res.Header().Get("To-Be-Changed"))
	assert.Equal(t, "added-value", res.Header().Get("Added-Response"))
	assert.Equal(t, "", res.Header().Get("To-Be-Removed"))

	mockLogger.AssertExpectations(t)
}

func TestHeaderTransformer_WriteWithoutExplicitStatusCode(t *testing.T) {
	// Setup
	mockLogger := &testutils.MockLogger{}
	mockLogger.On("Error", mock.Anything, mock.Anything).Maybe()

	transformer := middleware.NewHeaderTransformer(mockLogger)

	// Create a test handler that doesn't explicitly set status code
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Just write without setting status code
		w.Write([]byte("test"))
	})

	// Create header transform config
	transform := &config.HeaderTransform{
		Request: map[string]string{},
		Response: map[string]string{
			"Response-Header": "test-value",
		},
		Remove: []string{},
	}

	// Create the middleware with config
	middlewareHandler := transformer.Transform(testHandler, transform)

	// Create a test request and response recorder
	req := httptest.NewRequest("GET", "http://example.com", nil)
	res := httptest.NewRecorder()

	// Execute
	middlewareHandler.ServeHTTP(res, req)

	// Assert that default status 200 OK is used
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "test", res.Body.String())
	assert.Equal(t, "test-value", res.Header().Get("Response-Header"))

	mockLogger.AssertExpectations(t)
}
