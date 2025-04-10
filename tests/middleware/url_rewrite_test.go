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

func TestURLRewriter_BasicRewrite(t *testing.T) {
	// Create a mock logger
	mockLogger := new(testutils.MockLogger)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	// Create a new URL rewriter
	rewriter := middleware.NewURLRewriter(mockLogger)

	// Create a test HTTP handler that verifies the URL was rewritten
	var handlerCalled bool
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		// Check the URL was rewritten
		assert.Equal(t, "/api/products/1", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Create a test request with a URL that matches our pattern
	req := httptest.NewRequest("GET", "http://example.com/products/1", nil)
	rec := httptest.NewRecorder()

	// Define the rewrite rule
	rule := config.URLRewrite{
		Patterns: []config.URLRewritePattern{
			{
				Match:       "/products/(.*)",
				Replacement: "/api/products/$1",
			},
		},
	}

	// Create the middleware with the rule
	handler := rewriter.Rewrite(testHandler, &rule)

	// Process the request
	handler.ServeHTTP(rec, req)

	// Verify the response and that the handler was called
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test response", rec.Body.String())
	assert.True(t, handlerCalled)
}

func TestURLRewriter_NoMatch(t *testing.T) {
	// Create a mock logger
	mockLogger := new(testutils.MockLogger)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	// Create a new URL rewriter
	rewriter := middleware.NewURLRewriter(mockLogger)

	// Create a test HTTP handler that verifies the URL was not rewritten
	var handlerCalled bool
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		// URL should remain unchanged
		assert.Equal(t, "/other/path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Create a test request with a URL that doesn't match our pattern
	req := httptest.NewRequest("GET", "http://example.com/other/path", nil)
	rec := httptest.NewRecorder()

	// Define the rewrite rule
	rule := config.URLRewrite{
		Patterns: []config.URLRewritePattern{
			{
				Match:       "/products/(.*)",
				Replacement: "/api/products/$1",
			},
		},
	}

	// Create the middleware with the rule
	handler := rewriter.Rewrite(testHandler, &rule)

	// Process the request
	handler.ServeHTTP(rec, req)

	// Verify the response and that the handler was called
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test response", rec.Body.String())
	assert.True(t, handlerCalled)
}

func TestURLRewriter_ComplexRewrite(t *testing.T) {
	// Create a mock logger
	mockLogger := new(testutils.MockLogger)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	// Create a new URL rewriter
	rewriter := middleware.NewURLRewriter(mockLogger)

	// Create a test HTTP handler that verifies the URL was rewritten
	var handlerCalled bool
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		// Check the URL was rewritten with multiple capture groups
		assert.Equal(t, "/api/v2/users/123/posts/456", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Create a test request with a URL that matches our complex pattern
	req := httptest.NewRequest("GET", "http://example.com/users/123/posts/456", nil)
	rec := httptest.NewRecorder()

	// Define the rewrite rule with multiple capture groups
	rule := config.URLRewrite{
		Patterns: []config.URLRewritePattern{
			{
				Match:       "/users/(\\d+)/posts/(\\d+)",
				Replacement: "/api/v2/users/$1/posts/$2",
			},
		},
	}

	// Create the middleware with the rule
	handler := rewriter.Rewrite(testHandler, &rule)

	// Process the request
	handler.ServeHTTP(rec, req)

	// Verify the response and that the handler was called
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test response", rec.Body.String())
	assert.True(t, handlerCalled)
}

func TestURLRewriter_InvalidPattern(t *testing.T) {
	// Create a mock logger
	mockLogger := new(testutils.MockLogger)
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()

	// Create a new URL rewriter
	rewriter := middleware.NewURLRewriter(mockLogger)

	// Create a test HTTP handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// URL should remain unchanged due to invalid pattern
		assert.Equal(t, "/products/1", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Create a test request
	req := httptest.NewRequest("GET", "http://example.com/products/1", nil)
	rec := httptest.NewRecorder()

	// Define a rule with an invalid pattern that should cause an error
	rule := config.URLRewrite{
		Patterns: []config.URLRewritePattern{
			{
				Match:       "((invalid", // Invalid regex pattern
				Replacement: "/api/products/$1",
			},
		},
	}

	// Create the middleware with the rule
	handler := rewriter.Rewrite(testHandler, &rule)

	// Process the request - should pass through without rewriting
	handler.ServeHTTP(rec, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test response", rec.Body.String())

	// Verify that an error was logged
	mockLogger.AssertCalled(t, "Error", mock.Anything, mock.Anything)
}
