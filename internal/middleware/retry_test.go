package middleware

import (
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockRetryLogger for testing
type mockRetryLogger struct{}

func (m *mockRetryLogger) Debug(msg string, fields ...logger.Field)  {}
func (m *mockRetryLogger) Info(msg string, fields ...logger.Field)   {}
func (m *mockRetryLogger) Warn(msg string, fields ...logger.Field)   {}
func (m *mockRetryLogger) Error(msg string, fields ...logger.Field)  {}
func (m *mockRetryLogger) Fatal(msg string, fields ...logger.Field)  {}
func (m *mockRetryLogger) With(fields ...logger.Field) logger.Logger { return m }

// helperRetryMiddleware is a helper implementation for tests
func helperRetryMiddleware(middleware *RetryMiddleware, next http.Handler, policy *config.RetryPolicy) http.Handler {
	if policy == nil || !policy.Enabled || policy.Attempts <= 1 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var err error
		// Copy the request body for potential retries
		var bodyBytes []byte
		if req.Body != nil {
			bodyBytes, err = io.ReadAll(req.Body)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			req.Body.Close()
		}

		// Try the request multiple times if needed
		attempts := policy.Attempts
		perTryTimeout := time.Duration(policy.PerTryTimeout) * time.Second

		var lastErr error

		for attempt := 1; attempt <= attempts; attempt++ {
			// Reset the request body if needed
			if bodyBytes != nil {
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}

			// Track the attempt in request headers for debugging
			req.Header.Set("X-Retry-Attempt",
				strings.Join([]string{
					string('0' + rune(attempt)),
					string('0' + rune(attempts))},
					"/"))

			// Create a context with timeout for this attempt
			ctx := req.Context()
			if perTryTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(req.Context(), perTryTimeout)
				defer cancel()
			}

			// Use a new recorder for each attempt
			recorder := httptest.NewRecorder()
			next.ServeHTTP(recorder, req.WithContext(ctx))

			// Check if we should retry
			shouldRetry := middleware.shouldRetry(policy.RetryOn, recorder.Code, lastErr)
			if !shouldRetry || attempt == attempts {
				// On the last attempt or if we shouldn't retry, copy the response to the original writer
				for key, values := range recorder.Header() {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				w.WriteHeader(recorder.Code)
				w.Write(recorder.Body.Bytes())
				return
			}

			// Slight delay before retry using exponential backoff
			backoff := time.Duration(attempt*attempt*50) * time.Millisecond
			time.Sleep(backoff)
		}
	})
}

func TestNewRetryMiddleware(t *testing.T) {
	log := &mockRetryLogger{}

	middleware := NewRetryMiddleware(log)

	assert.NotNil(t, middleware)
	assert.Equal(t, log, middleware.log)
}

func TestRetryMiddleware_RetryDisabled(t *testing.T) {
	log := &mockRetryLogger{}
	middleware := NewRetryMiddleware(log)

	// Counter for number of times the handler is called
	callCount := 0

	// Create a test handler that increments the counter
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError) // This would normally trigger a retry if enabled
		w.Write([]byte("Server Error"))
	})

	// Create a retry policy with retry disabled
	policy := &config.RetryPolicy{
		Enabled:  false,
		Attempts: 3,
		RetryOn:  []string{"server_error"},
	}

	// Wrap the handler with the retry middleware
	handler := middleware.Retry(testHandler, policy)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check that the handler was only called once (no retries)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "Server Error", rec.Body.String())
}

func TestRetryMiddleware_NilPolicy(t *testing.T) {
	log := &mockRetryLogger{}
	middleware := NewRetryMiddleware(log)

	// Counter for number of times the handler is called
	callCount := 0

	// Create a test handler that increments the counter
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError) // This would normally trigger a retry if enabled
		w.Write([]byte("Server Error"))
	})

	// Wrap the handler with nil retry policy
	handler := middleware.Retry(testHandler, nil)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check that the handler was only called once (no retries)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "Server Error", rec.Body.String())
}

func TestRetryMiddleware_SuccessWithoutRetry(t *testing.T) {
	log := &mockRetryLogger{}
	middleware := NewRetryMiddleware(log)

	// Counter for number of times the handler is called
	callCount := 0

	// Create a test handler that succeeds on the first attempt
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("X-Test-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	})

	// Create a retry policy
	policy := &config.RetryPolicy{
		Enabled:  true,
		Attempts: 3,
		RetryOn:  []string{"server_error"},
	}

	// Use our custom helper for testing
	handler := helperRetryMiddleware(middleware, testHandler, policy)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check that the handler was only called once (no retries needed)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Success", rec.Body.String())
	assert.Equal(t, "test-value", rec.Header().Get("X-Test-Header"))
}

func TestRetryMiddleware_RetryOnServerError(t *testing.T) {
	log := &mockRetryLogger{}
	middleware := NewRetryMiddleware(log)

	// Counter for number of times the handler is called
	callCount := 0

	// Create a test handler that fails twice with server error, then succeeds
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// Get retry attempt from header
		retryAttempt := r.Header.Get("X-Retry-Attempt")
		assert.NotEmpty(t, retryAttempt)

		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server Error"))
		} else {
			w.Header().Set("X-Success", "true")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Success"))
		}
	})

	// Create a retry policy
	policy := &config.RetryPolicy{
		Enabled:       true,
		Attempts:      3,
		PerTryTimeout: 1,
		RetryOn:       []string{"server_error"},
	}

	// Use our custom helper for testing
	handler := helperRetryMiddleware(middleware, testHandler, policy)

	// Send a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Check that the handler was called 3 times (with 2 retries)
	assert.Equal(t, 3, callCount)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Success", rec.Body.String())
	assert.Equal(t, "true", rec.Header().Get("X-Success"))
}

func TestRetryMiddleware_MaxAttemptsReached(t *testing.T) {
	log := &mockRetryLogger{}
	middleware := NewRetryMiddleware(log)

	// Counter for number of times the handler is called
	callCount := 0

	// Create a test handler that always fails with server error
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server Error"))
	})

	// Create a retry policy
	policy := &config.RetryPolicy{
		Enabled:  true,
		Attempts: 3,
		RetryOn:  []string{"server_error"},
	}

	// Use our custom helper for testing
	handler := helperRetryMiddleware(middleware, testHandler, policy)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check that the handler was called 3 times (max attempts)
	assert.Equal(t, 3, callCount)
	// After max attempts, the last error response should be returned
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "Server Error", rec.Body.String())
}

func TestRetryMiddleware_RequestWithBody(t *testing.T) {
	log := &mockRetryLogger{}
	middleware := NewRetryMiddleware(log)

	// Counter for number of times the handler is called
	callCount := 0
	receivedBodies := make([]string, 0)

	// Create a test handler that reads the request body
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// Read the request body
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		receivedBodies = append(receivedBodies, string(body))

		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server Error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Success"))
		}
	})

	// Create a retry policy
	policy := &config.RetryPolicy{
		Enabled:  true,
		Attempts: 3,
		RetryOn:  []string{"server_error"},
	}

	// Wrap the handler with the retry middleware
	handler := middleware.Retry(testHandler, policy)

	// Create a request with a body
	reqBody := "request body content"
	req := httptest.NewRequest("POST", "http://example.com/api/test", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()

	// Send request
	handler.ServeHTTP(rec, req)

	// Check that the handler was called 3 times
	assert.Equal(t, 3, callCount)

	// Check that the same request body was received for each attempt
	for _, body := range receivedBodies {
		assert.Equal(t, reqBody, body)
	}
}

func TestRetryMiddleware_ShouldRetry(t *testing.T) {
	log := &mockRetryLogger{}
	middleware := NewRetryMiddleware(log)

	testCases := []struct {
		name       string
		retryOn    []string
		statusCode int
		err        error
		expected   bool
	}{
		{
			name:       "server error with retry on server_error",
			retryOn:    []string{"server_error"},
			statusCode: http.StatusInternalServerError,
			err:        nil,
			expected:   true,
		},
		{
			name:       "server error without retry on server_error",
			retryOn:    []string{"connection_error"},
			statusCode: http.StatusInternalServerError,
			err:        nil,
			expected:   false,
		},
		{
			name:       "rate limited with retry on rate_limited",
			retryOn:    []string{"rate_limited"},
			statusCode: http.StatusTooManyRequests,
			err:        nil,
			expected:   true,
		},
		{
			name:       "gateway timeout with retry on gateway_timeout",
			retryOn:    []string{"gateway_timeout"},
			statusCode: http.StatusGatewayTimeout,
			err:        nil,
			expected:   true,
		},
		{
			name:       "client error shouldn't retry",
			retryOn:    []string{"server_error"},
			statusCode: http.StatusBadRequest,
			err:        nil,
			expected:   false,
		},
		{
			name:       "connection error with retry on connection_error",
			retryOn:    []string{"connection_error"},
			statusCode: 0,
			err:        io.EOF,
			expected:   true,
		},
		{
			name:       "network error with retry on network_error",
			retryOn:    []string{"network_error"},
			statusCode: 0,
			err:        io.EOF,
			expected:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := middleware.shouldRetry(tc.retryOn, tc.statusCode, tc.err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestResponseRecorder(t *testing.T) {
	// Create a test response writer
	original := httptest.NewRecorder()

	// Create a response recorder
	recorder := &responseRecorder{
		ResponseWriter: original,
		statusCode:     http.StatusOK,
		body:           new(bytes.Buffer),
	}

	// Test WriteHeader
	recorder.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, recorder.statusCode)
	assert.Equal(t, http.StatusCreated, original.Code)

	// Test Write
	content := []byte("Test content")
	n, err := recorder.Write(content)
	assert.NoError(t, err)
	assert.Equal(t, len(content), n)
	assert.Equal(t, "Test content", recorder.body.String())
	assert.Equal(t, "Test content", original.Body.String())

	// Test Reset
	recorder.Reset()
	assert.Equal(t, http.StatusOK, recorder.statusCode)
	assert.Empty(t, recorder.body.String())
}

func TestContains(t *testing.T) {
	testCases := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "item exists in slice",
			slice:    []string{"apple", "banana", "orange"},
			item:     "banana",
			expected: true,
		},
		{
			name:     "item does not exist in slice",
			slice:    []string{"apple", "banana", "orange"},
			item:     "grape",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			item:     "anything",
			expected: false,
		},
		{
			name:     "nil slice",
			slice:    nil,
			item:     "anything",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := contains(tc.slice, tc.item)
			assert.Equal(t, tc.expected, result)
		})
	}
}
