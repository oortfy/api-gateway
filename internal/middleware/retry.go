package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
)

// RetryMiddleware provides retry functionality for failed requests
type RetryMiddleware struct {
	log logger.Logger
}

// NewRetryMiddleware creates a new retry middleware
func NewRetryMiddleware(log logger.Logger) *RetryMiddleware {
	return &RetryMiddleware{
		log: log,
	}
}

// Retry wraps a handler with retry logic
func (r *RetryMiddleware) Retry(next http.Handler, policy *config.RetryPolicy) http.Handler {
	if policy == nil || !policy.Enabled || policy.Attempts <= 1 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Create a response recorder to capture the response
		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           new(bytes.Buffer),
		}

		var err error
		// Copy the request body for potential retries
		var bodyBytes []byte
		if req.Body != nil {
			bodyBytes, err = io.ReadAll(req.Body)
			if err != nil {
				r.log.Error("Failed to read request body",
					logger.String("path", req.URL.Path),
					logger.Error(err),
				)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			req.Body.Close()
		}

		// Try the request multiple times if needed
		attempts := policy.Attempts
		perTryTimeout := time.Duration(policy.PerTryTimeout) * time.Second

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

			// Serve the request with the new context
			recorder.Reset()
			next.ServeHTTP(recorder, req.WithContext(ctx))

			// Check if we should retry
			shouldRetry := r.shouldRetry(policy.RetryOn, recorder.statusCode, err)
			if !shouldRetry || attempt == attempts {
				// On the last attempt or if we shouldn't retry, copy the response to the original writer
				for key, values := range recorder.Header() {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				w.WriteHeader(recorder.statusCode)
				if recorder.body != nil {
					w.Write(recorder.body.Bytes())
				}
				return
			}

			r.log.Debug("Retrying request",
				logger.String("path", req.URL.Path),
				logger.Int("attempt", attempt),
				logger.Int("max_attempts", attempts),
				logger.Int("status_code", recorder.statusCode),
			)

			// Slight delay before retry using exponential backoff
			backoff := time.Duration(attempt*attempt*50) * time.Millisecond
			time.Sleep(backoff)
		}
	})
}

// shouldRetry determines if a request should be retried based on the retry policy
func (r *RetryMiddleware) shouldRetry(retryOn []string, statusCode int, err error) bool {
	// If there was a network error, retry
	if err != nil {
		return contains(retryOn, "connection_error") || contains(retryOn, "network_error")
	}

	// Check status code based retry conditions
	if statusCode >= 500 && contains(retryOn, "server_error") {
		return true
	}

	if statusCode == http.StatusTooManyRequests && contains(retryOn, "rate_limited") {
		return true
	}

	if statusCode == http.StatusGatewayTimeout && contains(retryOn, "gateway_timeout") {
		return true
	}

	return false
}

// contains checks if a string slice contains a specific value
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// responseRecorder is a wrapper around http.ResponseWriter that records the response
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// WriteHeader captures the status code
func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response body
func (r *responseRecorder) Write(b []byte) (int, error) {
	// Ensure the body buffer is initialized
	if r.body == nil {
		r.body = new(bytes.Buffer)
	}

	// Write to the underlying response writer as well
	r.ResponseWriter.Write(b)

	return r.body.Write(b)
}

// Header returns the header map that will be sent
func (r *responseRecorder) Header() http.Header {
	return r.ResponseWriter.Header()
}

// Reset clears the recorder for reuse
func (r *responseRecorder) Reset() {
	r.statusCode = http.StatusOK
	if r.body != nil {
		r.body.Reset()
	} else {
		r.body = new(bytes.Buffer)
	}
}
