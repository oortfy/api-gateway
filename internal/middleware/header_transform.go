package middleware

import (
	"net/http"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
)

// HeaderTransformer handles transformation of HTTP headers
type HeaderTransformer struct {
	log logger.Logger
}

// NewHeaderTransformer creates a new header transformation middleware
func NewHeaderTransformer(log logger.Logger) *HeaderTransformer {
	return &HeaderTransformer{
		log: log,
	}
}

// Transform applies header transformations based on configuration
func (h *HeaderTransformer) Transform(next http.Handler, transform *config.HeaderTransform) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if transform == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Apply request header transformations
		for key, value := range transform.Request {
			r.Header.Set(key, value)
		}

		// Create a custom response writer to handle response header transformations
		tw := &transformResponseWriter{
			ResponseWriter: w,
			transform:      transform,
			log:            h.log,
		}

		// Continue to next handler with our custom response writer
		next.ServeHTTP(tw, r)
	})
}

// transformResponseWriter is a wrapper for http.ResponseWriter that
// applies header transformations to responses
type transformResponseWriter struct {
	http.ResponseWriter
	transform   *config.HeaderTransform
	log         logger.Logger
	wroteHeader bool
}

// WriteHeader overrides the original WriteHeader to apply header transformations
func (tw *transformResponseWriter) WriteHeader(statusCode int) {
	if tw.wroteHeader {
		return
	}
	tw.wroteHeader = true

	// Apply response header transformations
	for key, value := range tw.transform.Response {
		// Empty value means remove the header
		if value == "" {
			tw.ResponseWriter.Header().Del(key)
		} else {
			tw.ResponseWriter.Header().Set(key, value)
		}
	}

	// Remove headers if specified
	for _, header := range tw.transform.Remove {
		tw.ResponseWriter.Header().Del(header)
	}

	tw.ResponseWriter.WriteHeader(statusCode)
}

// Write overrides the original Write method to ensure headers are written
func (tw *transformResponseWriter) Write(b []byte) (int, error) {
	if !tw.wroteHeader {
		tw.WriteHeader(http.StatusOK)
	}
	return tw.ResponseWriter.Write(b)
}
