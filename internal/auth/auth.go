package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"

	"github.com/golang-jwt/jwt/v4"
)

var (
	ErrNoToken      = errors.New("no authentication token provided")
	ErrInvalidToken = errors.New("invalid authentication token")
	ErrExpiredToken = errors.New("token has expired")
	ErrForbidden    = errors.New("forbidden: insufficient permissions")
	ErrAuthFailed   = errors.New("authentication failed")
)

// AuthService provides authentication functionality
type AuthService struct {
	config *config.AuthConfig
	log    logger.Logger
	client *http.Client
}

// APIKeyResponse represents the response from the API key validation endpoint
type APIKeyResponse struct {
	Valid       bool     `json:"valid"`
	UserID      string   `json:"user_id"`
	TenantID    string   `json:"tenant_id"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	AuthType    string   `json:"auth_type"`
}

// JWTClaims represents the custom claims for the JWT
type JWTClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// NewAuthService creates a new authentication service
func NewAuthService(config *config.AuthConfig, log logger.Logger) *AuthService {
	return &AuthService{
		config: config,
		log:    log,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// ValidateToken validates the provided authentication token
// It first tries to validate as a JWT token, if that fails, it tries as an API token
func (a *AuthService) ValidateToken(r *http.Request, allowedRoles []string) (bool, error) {
	var jwtToken, apiToken string

	// First look in headers
	jwtToken = a.extractJWTToken(r)
	apiToken = a.extractAPIToken(r)

	// If not found in headers, try query parameters
	if jwtToken == "" {
		jwtToken = a.extractJWTTokenFromQuery(r)
		a.log.Debug("Extracted JWT token from query parameters",
			logger.String("path", r.URL.Path),
			logger.Bool("token_found", jwtToken != ""),
		)
	}

	if apiToken == "" {
		apiToken = a.extractAPITokenFromQuery(r)
		a.log.Debug("Extracted API token from query parameters",
			logger.String("path", r.URL.Path),
			logger.Bool("token_found", apiToken != ""),
		)
	}

	// Try JWT validation first
	if jwtToken != "" {
		valid, _, err := a.validateJWT(jwtToken)
		if err == nil && valid {
			// Skip role checking - any authenticated user is allowed
			return true, nil
		}

		// If it's a definite error like malformed JWT, return immediately
		if err != nil && !errors.Is(err, ErrNoToken) {
			a.log.Debug("JWT validation failed", logger.Error(err))
			return false, err
		}
	}

	// Try API token validation next
	if apiToken != "" {
		valid, _, err := a.validateAPIToken(apiToken)
		if err != nil {
			a.log.Debug("API token validation failed", logger.Error(err))
			return false, err
		}
		if valid {
			// Skip role checking - any authenticated user is allowed
			return true, nil
		}
	}

	// Neither token type was valid
	if jwtToken == "" && apiToken == "" {
		return false, ErrNoToken
	}

	return false, ErrAuthFailed
}

// extractJWTToken extracts JWT token from the Authorization header
func (a *AuthService) extractJWTToken(r *http.Request) string {
	authHeader := r.Header.Get(a.config.JWTHeader)
	if authHeader == "" {
		return ""
	}

	// Check if the header has the "Bearer " prefix
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}

// extractJWTTokenFromQuery extracts JWT token from the URL query parameters
func (a *AuthService) extractJWTTokenFromQuery(r *http.Request) string {
	// Check for token parameter
	token := r.URL.Query().Get("token")
	if token != "" {
		return token
	}

	// Also check for access_token parameter (OAuth 2.0 standard)
	return r.URL.Query().Get("access_token")
}

// extractAPIToken extracts API token from the header
func (a *AuthService) extractAPIToken(r *http.Request) string {
	return r.Header.Get(a.config.APIKeyHeader)
}

// extractAPITokenFromQuery extracts API token from the URL query parameters
func (a *AuthService) extractAPITokenFromQuery(r *http.Request) string {
	// Check for api_key parameter
	apiKey := r.URL.Query().Get("api_key")
	if apiKey != "" {
		return apiKey
	}

	// Also check for key parameter (common alternative)
	return r.URL.Query().Get("key")
}

// validateJWT validates a JWT token and returns the associated role
func (a *AuthService) validateJWT(tokenString string) (bool, string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the algorithm
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.JWTSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return false, "", ErrExpiredToken
		}
		return false, "", ErrInvalidToken
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return true, claims.Role, nil
	}

	return false, "", ErrInvalidToken
}

// validateAPIToken validates an API token by making a request to the validation endpoint
func (a *AuthService) validateAPIToken(token string) (bool, string, error) {
	if a.config.APIKeyValidationURL == "" {
		return false, "", errors.New("API key validation URL not configured")
	}

	// Create a new HTTP request according to the specified format
	req, err := http.NewRequest(http.MethodPost, a.config.APIKeyValidationURL, nil)
	if err != nil {
		return false, "", err
	}

	// Set the x-api-key header instead of Authorization header
	req.Header.Set("x-api-key", token)
	req.Header.Set("accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("API key validation request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response status code is successful
	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("API key validation failed with status: %d", resp.StatusCode)
	}

	// Parse the response body
	var apiKeyResp APIKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiKeyResp); err != nil {
		return false, "", fmt.Errorf("failed to decode API key validation response: %w", err)
	}

	if !apiKeyResp.Valid {
		return false, "", errors.New("invalid API key")
	}

	// Return the validation result and role
	return true, apiKeyResp.Role, nil
}

// checkRole checks if the provided role is in the list of allowed roles
func (a *AuthService) checkRole(role string, allowedRoles []string) (bool, error) {
	// If no specific roles are required, any authenticated user is allowed
	if len(allowedRoles) == 0 {
		return true, nil
	}

	// Check if "any" is in the allowed roles list - this is a wildcard that matches any role
	for _, allowedRole := range allowedRoles {
		if allowedRole == "any" {
			return true, nil
		}
	}

	// Check if the user's specific role is allowed
	for _, allowedRole := range allowedRoles {
		if allowedRole == role {
			return true, nil
		}
	}

	return false, ErrForbidden
}
