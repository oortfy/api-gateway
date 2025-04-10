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

		// Set allowed origin
		useWildcard := c.config.AllowAllOrigins ||
			(len(c.config.AllowedOrigins) == 1 && c.config.AllowedOrigins[0] == "*" &&
				!c.config.AllowCredentials)

		if useWildcard {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		// Set other CORS headers
		if c.config.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if len(c.config.ExposedHeaders) > 0 {
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(c.config.ExposedHeaders, ","))
		}

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			requestMethod := r.Header.Get("Access-Control-Request-Method")
			if requestMethod != "" {
				// Set preflight headers
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(c.config.AllowedMethods, ","))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(c.config.AllowedHeaders, ","))
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(c.config.MaxAge))

				c.log.Info("CORS preflight request processed",
					logger.String("origin", origin),
					logger.String("method", requestMethod),
				)

				// Preflight request completed, no need to call the next handler
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		// Continue with the request
		next.ServeHTTP(w, r)
	})
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
