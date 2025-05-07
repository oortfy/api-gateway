package middleware

import (
	"net/http"

	"api-gateway/internal/auth"
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
)

// AuthMiddleware provides authentication middleware functionality
type AuthMiddleware struct {
	authService *auth.AuthService
	authConfig  *config.AuthConfig
	log         logger.Logger
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(authService *auth.AuthService, authConfig *config.AuthConfig, log logger.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
		authConfig:  authConfig,
		log:         log,
	}
}

// safeError is a helper function to safely write error responses
func safeError(w http.ResponseWriter, msg string, statusCode int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)
	// We ignore any write errors as there's not much we can do about them
	w.Write([]byte(msg))
}

// Authenticate checks if the request has valid authentication
func (m *AuthMiddleware) Authenticate(next http.Handler, route config.Route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication if not required for this route
		if !route.Middlewares.RequireAuth {
			next.ServeHTTP(w, r)
			return
		}

		// Skip authentication for OPTIONS requests (CORS preflight)
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Extract the API key from headers if present
		apiKey := r.Header.Get("x-api-key")
		if apiKey != "" {
			// If API key is present, set it to the expected header name
			r.Header.Set(m.authConfig.APIKeyHeader, apiKey)
		}

		// Validate the token - passing empty slice for allowedRoles to skip role checking
		valid, err := m.authService.ValidateToken(r, []string{})
		if err != nil {
			m.log.Debug("Authentication failed",
				logger.String("path", r.URL.Path),
				logger.String("method", r.Method),
				logger.Error(err),
			)

			// Send appropriate error response using our safe error function
			switch err {
			case auth.ErrNoToken:
				safeError(w, "Authorization required", http.StatusUnauthorized)
			case auth.ErrInvalidToken, auth.ErrExpiredToken:
				safeError(w, err.Error(), http.StatusUnauthorized)
			case auth.ErrForbidden:
				safeError(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
			default:
				safeError(w, "Authentication failed", http.StatusUnauthorized)
			}
			return
		}

		if !valid {
			m.log.Debug("Authentication invalid",
				logger.String("path", r.URL.Path),
				logger.String("method", r.Method),
			)
			safeError(w, "Authentication failed", http.StatusUnauthorized)
			return
		}

		// Authentication succeeded, continue to the next handler
		next.ServeHTTP(w, r)
	})
}
