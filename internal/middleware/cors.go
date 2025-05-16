package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
)

// CORSMiddleware provides CORS functionality
type CORSMiddleware struct {
	config *config.CORSConfig
	log    logger.Logger
}

// NewCORSMiddleware creates a new CORS middleware
func NewCORSMiddleware(config *config.CORSConfig, log logger.Logger) *CORSMiddleware {
	return &CORSMiddleware{
		config: config,
		log:    log,
	}
}

// CORS middleware handles Cross-Origin Resource Sharing
func (c *CORSMiddleware) CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If CORS is disabled, just pass through
		if !c.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			// Not a CORS request, continue
			next.ServeHTTP(w, r)
			return
		}

		// Check if the origin is allowed
		if !c.isOriginAllowed(origin) {
			// Origin not allowed, continue without CORS headers
			next.ServeHTTP(w, r)
			return
		}

		// Create a wrapper for the response writer to capture and modify headers
		wrapper := &corsResponseWriter{
			ResponseWriter: w,
			config:         c.config,
			origin:         origin,
			log:            c.log,
		}

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			requestMethod := r.Header.Get("Access-Control-Request-Method")
			if requestMethod != "" {
				// Process preflight request
				wrapper.handlePreflight(requestMethod)
				return
			}
		}

		// Continue with the request using our wrapper
		next.ServeHTTP(wrapper, r)
	})
}

// corsResponseWriter wraps http.ResponseWriter to handle CORS headers
type corsResponseWriter struct {
	http.ResponseWriter
	config *config.CORSConfig
	origin string
	log    logger.Logger
}

// handlePreflight processes OPTIONS preflight requests
func (w *corsResponseWriter) handlePreflight(requestMethod string) {
	// Set CORS headers for preflight
	w.setCORSHeaders()

	// Set method-specific preflight headers
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(w.config.AllowedMethods, ","))
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(w.config.AllowedHeaders, ","))
	w.Header().Set("Access-Control-Max-Age", strconv.Itoa(w.config.MaxAge))

	w.log.Info("CORS preflight request processed",
		logger.String("origin", w.origin),
		logger.String("method", requestMethod),
	)

	// Preflight request completed
	w.WriteHeader(http.StatusOK)
}

// WriteHeader overrides the original WriteHeader to ensure CORS headers are set first
func (w *corsResponseWriter) WriteHeader(statusCode int) {
	w.setCORSHeaders()
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write overrides the original Write to ensure CORS headers are set
func (w *corsResponseWriter) Write(b []byte) (int, error) {
	w.setCORSHeaders()
	return w.ResponseWriter.Write(b)
}

// Header returns the header map to use for the response
func (w *corsResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

// setCORSHeaders sets the CORS headers if they aren't already set
func (w *corsResponseWriter) setCORSHeaders() {
	// If the Access-Control-Allow-Origin header is already set, don't override it
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		return
	}

	// Set allowed origin
	useWildcard := w.config.AllowAllOrigins ||
		(len(w.config.AllowedOrigins) == 1 && w.config.AllowedOrigins[0] == "*" &&
			!w.config.AllowCredentials)

	if useWildcard {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", w.origin)
		w.Header().Set("Vary", "Origin")
	}

	// Set other CORS headers
	if w.config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if len(w.config.ExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(w.config.ExposedHeaders, ","))
	}
}

// isOriginAllowed checks if the origin is allowed
func (c *CORSMiddleware) isOriginAllowed(origin string) bool {
	if c.config.AllowAllOrigins {
		return true
	}

	for _, allowedOrigin := range c.config.AllowedOrigins {
		if allowedOrigin == "*" {
			return true
		}
		if allowedOrigin == origin {
			return true
		}
	}

	return false
}
