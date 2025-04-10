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

// Authenticate checks if the request has valid authentication
func (m *AuthMiddleware) Authenticate(next http.Handler, route config.Route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication if not required for this route
		if !route.RequireAuth {
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

			// Send appropriate error response
			switch err {
			case auth.ErrNoToken:
				http.Error(w, "Authorization required", http.StatusUnauthorized)
			case auth.ErrInvalidToken, auth.ErrExpiredToken:
				http.Error(w, err.Error(), http.StatusUnauthorized)
			case auth.ErrForbidden:
				http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
			default:
				http.Error(w, "Authentication failed", http.StatusUnauthorized)
			}
			return
		}

		if !valid {
			m.log.Debug("Authentication invalid",
				logger.String("path", r.URL.Path),
				logger.String("method", r.Method),
			)
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
			return
		}

		// Authentication succeeded, continue to the next handler
		next.ServeHTTP(w, r)
	})
}
