package middleware

import (
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockURLRewriteLogger for testing
type mockURLRewriteLogger struct{}

func (m *mockURLRewriteLogger) Debug(msg string, fields ...logger.Field)  {}
func (m *mockURLRewriteLogger) Info(msg string, fields ...logger.Field)   {}
func (m *mockURLRewriteLogger) Warn(msg string, fields ...logger.Field)   {}
func (m *mockURLRewriteLogger) Error(msg string, fields ...logger.Field)  {}
func (m *mockURLRewriteLogger) Fatal(msg string, fields ...logger.Field)  {}
func (m *mockURLRewriteLogger) With(fields ...logger.Field) logger.Logger { return m }

func TestNewURLRewriter(t *testing.T) {
	log := &mockURLRewriteLogger{}

	rewriter := NewURLRewriter(log)

	assert.NotNil(t, rewriter)
	assert.Equal(t, log, rewriter.log)
}

func TestURLRewriter_NilConfig(t *testing.T) {
	log := &mockURLRewriteLogger{}
	rewriter := NewURLRewriter(log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the URL path back in the response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.URL.Path))
	})

	// Wrap the handler with nil rewrite configuration
	handler := rewriter.Rewrite(testHandler, nil)

	// Create a request with a test path
	originalPath := "/api/users/123"
	req := httptest.NewRequest("GET", "http://example.com"+originalPath, nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check that path was not modified
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, originalPath, rec.Body.String())
}

func TestURLRewriter_EmptyPatterns(t *testing.T) {
	log := &mockURLRewriteLogger{}
	rewriter := NewURLRewriter(log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the URL path back in the response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.URL.Path))
	})

	// Wrap the handler with empty patterns configuration
	handler := rewriter.Rewrite(testHandler, &config.URLRewrite{
		Patterns: []config.URLRewritePattern{},
	})

	// Create a request with a test path
	originalPath := "/api/users/123"
	req := httptest.NewRequest("GET", "http://example.com"+originalPath, nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check that path was not modified
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, originalPath, rec.Body.String())
}

func TestURLRewriter_InvalidPattern(t *testing.T) {
	log := &mockURLRewriteLogger{}
	rewriter := NewURLRewriter(log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the URL path back in the response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.URL.Path))
	})

	// Wrap the handler with invalid regex pattern
	handler := rewriter.Rewrite(testHandler, &config.URLRewrite{
		Patterns: []config.URLRewritePattern{
			{
				Match:       "[", // Invalid regex pattern
				Replacement: "/new",
			},
		},
	})

	// Create a request with a test path
	originalPath := "/api/users/123"
	req := httptest.NewRequest("GET", "http://example.com"+originalPath, nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check that path was not modified (invalid pattern should be skipped)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, originalPath, rec.Body.String())
}

func TestURLRewriter_BasicRewrite(t *testing.T) {
	log := &mockURLRewriteLogger{}
	rewriter := NewURLRewriter(log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the URL path back in the response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.URL.Path))
	})

	// Wrap the handler with a simple rewrite pattern
	handler := rewriter.Rewrite(testHandler, &config.URLRewrite{
		Patterns: []config.URLRewritePattern{
			{
				Match:       "^/api/users/(.*)$",
				Replacement: "/users/$1",
			},
		},
	})

	// Create a request with a path that matches the pattern
	req := httptest.NewRequest("GET", "http://example.com/api/users/123", nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check that path was rewritten
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/users/123", rec.Body.String())
}

func TestURLRewriter_MultiplePatterns(t *testing.T) {
	log := &mockURLRewriteLogger{}
	rewriter := NewURLRewriter(log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the URL path back in the response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.URL.Path))
	})

	// Wrap the handler with multiple rewrite patterns
	handler := rewriter.Rewrite(testHandler, &config.URLRewrite{
		Patterns: []config.URLRewritePattern{
			{
				Match:       "^/api/users/(.*)$",
				Replacement: "/users/$1",
			},
			{
				Match:       "^/api/products/(.*)$",
				Replacement: "/products/$1",
			},
			{
				Match:       "^/api/orders/(.*)$",
				Replacement: "/orders/$1",
			},
		},
	})

	// Test case 1: First pattern should match
	req1 := httptest.NewRequest("GET", "http://example.com/api/users/123", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	assert.Equal(t, "/users/123", rec1.Body.String())

	// Test case 2: Second pattern should match
	req2 := httptest.NewRequest("GET", "http://example.com/api/products/456", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, "/products/456", rec2.Body.String())

	// Test case 3: Third pattern should match
	req3 := httptest.NewRequest("GET", "http://example.com/api/orders/789", nil)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	assert.Equal(t, "/orders/789", rec3.Body.String())

	// Test case 4: No pattern should match
	req4 := httptest.NewRequest("GET", "http://example.com/api/other/123", nil)
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4)
	assert.Equal(t, "/api/other/123", rec4.Body.String())
}

func TestURLRewriter_ComplexRewrite(t *testing.T) {
	log := &mockURLRewriteLogger{}
	rewriter := NewURLRewriter(log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the URL path back in the response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.URL.Path))
	})

	// Wrap the handler with a complex rewrite pattern (with multiple capture groups)
	handler := rewriter.Rewrite(testHandler, &config.URLRewrite{
		Patterns: []config.URLRewritePattern{
			{
				Match:       "^/api/v(\\d+)/users/(\\d+)/profile$",
				Replacement: "/v$1/profiles/user/$2",
			},
		},
	})

	// Test the complex rewrite pattern
	req := httptest.NewRequest("GET", "http://example.com/api/v2/users/42/profile", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/v2/profiles/user/42", rec.Body.String())
}

func TestURLRewriter_FirstMatchOnly(t *testing.T) {
	log := &mockURLRewriteLogger{}
	rewriter := NewURLRewriter(log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the URL path back in the response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.URL.Path))
	})

	// Wrap the handler with multiple matching patterns
	handler := rewriter.Rewrite(testHandler, &config.URLRewrite{
		Patterns: []config.URLRewritePattern{
			{
				Match:       "^/api/users/.*$",
				Replacement: "/first-match",
			},
			{
				Match:       "^/api/users/\\d+$",
				Replacement: "/second-match",
			},
		},
	})

	// The request should match both patterns, but only the first should be applied
	req := httptest.NewRequest("GET", "http://example.com/api/users/123", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/first-match", rec.Body.String())
}
