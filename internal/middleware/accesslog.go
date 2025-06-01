package middleware

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
)

// AccessLogMiddleware provides request and response logging
type AccessLogMiddleware struct {
	config *config.LoggingConfig
	log    logger.Logger
}

// NewAccessLogMiddleware creates a new access log middleware
func NewAccessLogMiddleware(config *config.LoggingConfig, log logger.Logger) *AccessLogMiddleware {
	return &AccessLogMiddleware{
		config: config,
		log:    log,
	}
}

// accessLogResponseWriter is a wrapper for http.ResponseWriter that captures response data
type accessLogResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	logBodies  bool
}

// WriteHeader captures the status code before writing it
func (w *accessLogResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Write captures the response body before writing it
func (w *accessLogResponseWriter) Write(b []byte) (int, error) {
	// Only capture body if we're logging bodies
	if w.logBodies {
		w.body.Write(b)
	}
	// Then write to the original ResponseWriter
	return w.ResponseWriter.Write(b)
}

// AccessLog middleware logs requests and responses
func (a *AccessLogMiddleware) AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip access logging if not enabled
		if !a.config.EnableAccess {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()

		// Only log request bodies in debug mode with level set to "debug"
		logBodies := a.config.Level == "debug"

		// Read and log the request body only if in debug mode
		if logBodies && r.Body != nil {
			// Clone the request body so we can read it without consuming it
			reqBody, _ := io.ReadAll(r.Body)
			r.Body.Close()

			// Create a new ReadCloser to replace the original body
			r.Body = io.NopCloser(bytes.NewBuffer(reqBody))

			// Log the request body (truncate if too large)
			if len(reqBody) > 0 {
				if len(reqBody) > 1024 {
					a.log.Debug("Request Body (truncated)",
						logger.String("method", r.Method),
						logger.String("path", r.URL.Path),
						logger.String("body", string(reqBody[:1024])+"..."),
					)
				} else {
					a.log.Debug("Request Body",
						logger.String("method", r.Method),
						logger.String("path", r.URL.Path),
						logger.String("body", string(reqBody)),
					)
				}
			}
		}

		// Create a custom response writer to capture the response
		writer := &accessLogResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           new(bytes.Buffer),
			logBodies:      logBodies,
		}

		// Process the request
		next.ServeHTTP(writer, r)

		// Calculate duration
		duration := time.Since(start).Milliseconds()

		// Log the response body only in debug mode
		if logBodies && writer.body.Len() > 0 {
			respBody := writer.body.Bytes()
			if len(respBody) > 1024 {
				a.log.Debug("Response Body (truncated)",
					logger.String("method", r.Method),
					logger.String("path", r.URL.Path),
					logger.Int("status", writer.statusCode),
					logger.String("body", string(respBody[:1024])+"..."),
				)
			} else {
				a.log.Debug("Response Body",
					logger.String("method", r.Method),
					logger.String("path", r.URL.Path),
					logger.Int("status", writer.statusCode),
					logger.String("body", string(respBody)),
				)
			}
		}

		// Single concise log line for the entire request/response cycle
		a.log.Info("API Request",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.Int("status", writer.statusCode),
			logger.Int("duration_ms", int(duration)),
			logger.Int("size", writer.body.Len()),
			logger.String("remote", r.RemoteAddr),
		)
	})
}
