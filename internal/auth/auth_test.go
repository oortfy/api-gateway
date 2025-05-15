package auth

import (
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implements the logger.Logger interface for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...logger.Field)  {}
func (m *mockLogger) Info(msg string, fields ...logger.Field)   {}
func (m *mockLogger) Warn(msg string, fields ...logger.Field)   {}
func (m *mockLogger) Error(msg string, fields ...logger.Field)  {}
func (m *mockLogger) Fatal(msg string, fields ...logger.Field)  {}
func (m *mockLogger) With(fields ...logger.Field) logger.Logger { return m }

func TestNewAuthService(t *testing.T) {
	cfg := &config.AuthConfig{
		JWTSecret:           "test-secret",
		JWTHeader:           "Authorization",
		APIKeyHeader:        "X-API-Key",
		APIKeyValidationURL: "https://test.com/validate",
	}
	log := &mockLogger{}

	svc := NewAuthService(cfg, log)
	assert.NotNil(t, svc)
	assert.Equal(t, cfg, svc.config)
	assert.Equal(t, log, svc.log)
	assert.NotNil(t, svc.client)
	assert.Equal(t, 5*time.Second, svc.client.Timeout)
}

func createTestJWT(t *testing.T, secret, role string, expiry time.Time) string {
	claims := &JWTClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   "test-user",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signedToken
}

func TestExtractJWTToken(t *testing.T) {
	cfg := &config.AuthConfig{
		JWTHeader: "Authorization",
	}
	svc := &AuthService{config: cfg, log: &mockLogger{}}

	tests := []struct {
		name      string
		headerVal string
		wantToken string
	}{
		{
			name:      "valid bearer token",
			headerVal: "Bearer test-token",
			wantToken: "test-token",
		},
		{
			name:      "missing bearer prefix",
			headerVal: "test-token",
			wantToken: "",
		},
		{
			name:      "empty header",
			headerVal: "",
			wantToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/test", nil)
			if tt.headerVal != "" {
				req.Header.Set("Authorization", tt.headerVal)
			}
			token := svc.extractJWTToken(req)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}

func TestExtractJWTTokenFromQuery(t *testing.T) {
	svc := &AuthService{log: &mockLogger{}}

	tests := []struct {
		name        string
		queryParams string
		wantToken   string
	}{
		{
			name:        "token parameter",
			queryParams: "token=test-token",
			wantToken:   "test-token",
		},
		{
			name:        "access_token parameter",
			queryParams: "access_token=test-token",
			wantToken:   "test-token",
		},
		{
			name:        "no token parameters",
			queryParams: "other=value",
			wantToken:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/test?"+tt.queryParams, nil)
			token := svc.extractJWTTokenFromQuery(req)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}

func TestValidateJWT(t *testing.T) {
	secret := "test-secret"
	svc := &AuthService{
		config: &config.AuthConfig{JWTSecret: secret},
		log:    &mockLogger{},
	}

	// Valid token
	validToken := createTestJWT(t, secret, "admin", time.Now().Add(time.Hour))

	// Expired token
	expiredToken := createTestJWT(t, secret, "user", time.Now().Add(-time.Hour))

	// Invalid signature
	invalidToken := createTestJWT(t, "wrong-secret", "user", time.Now().Add(time.Hour))

	tests := []struct {
		name      string
		token     string
		wantValid bool
		wantRole  string
		wantErr   error
	}{
		{
			name:      "valid token",
			token:     validToken,
			wantValid: true,
			wantRole:  "admin",
			wantErr:   nil,
		},
		{
			name:      "expired token",
			token:     expiredToken,
			wantValid: false,
			wantRole:  "",
			wantErr:   ErrExpiredToken,
		},
		{
			name:      "invalid signature",
			token:     invalidToken,
			wantValid: false,
			wantRole:  "",
			wantErr:   ErrInvalidToken,
		},
		{
			name:      "empty token",
			token:     "",
			wantValid: false,
			wantRole:  "",
			wantErr:   ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, role, err := svc.validateJWT(tt.token)
			assert.Equal(t, tt.wantValid, valid)
			assert.Equal(t, tt.wantRole, role)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAPIToken(t *testing.T) {
	// Create a test server to mock the API key validation endpoint
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("x-api-key")

		var resp APIKeyResponse
		switch apiKey {
		case "valid-key":
			resp = APIKeyResponse{
				Valid:    true,
				UserID:   "user123",
				TenantID: "tenant456",
				Role:     "admin",
			}
		default:
			resp = APIKeyResponse{Valid: false}
			w.WriteHeader(http.StatusUnauthorized)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	svc := &AuthService{
		config: &config.AuthConfig{
			APIKeyValidationURL: ts.URL,
		},
		log:    &mockLogger{},
		client: &http.Client{Timeout: 5 * time.Second},
	}

	tests := []struct {
		name      string
		token     string
		wantValid bool
		wantRole  string
		wantErr   bool
	}{
		{
			name:      "valid API key",
			token:     "valid-key",
			wantValid: true,
			wantRole:  "admin",
			wantErr:   false,
		},
		{
			name:      "invalid API key",
			token:     "invalid-key",
			wantValid: false,
			wantRole:  "",
			wantErr:   true,
		},
		{
			name:      "empty API key",
			token:     "",
			wantValid: false,
			wantRole:  "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, role, err := svc.validateAPIToken(tt.token)
			assert.Equal(t, tt.wantValid, valid)
			assert.Equal(t, tt.wantRole, role)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	secret := "test-secret"

	// Create a test server to mock the API key validation endpoint
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("x-api-key")

		var resp APIKeyResponse
		if apiKey == "valid-api-key" {
			resp = APIKeyResponse{
				Valid:    true,
				UserID:   "user123",
				TenantID: "tenant456",
				Role:     "admin",
			}
			w.WriteHeader(http.StatusOK)
		} else {
			resp = APIKeyResponse{Valid: false}
			w.WriteHeader(http.StatusUnauthorized)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	svc := &AuthService{
		config: &config.AuthConfig{
			JWTSecret:           secret,
			JWTHeader:           "Authorization",
			APIKeyHeader:        "X-API-Key",
			APIKeyValidationURL: ts.URL,
		},
		log:    &mockLogger{},
		client: &http.Client{Timeout: 5 * time.Second},
	}

	validJWT := createTestJWT(t, secret, "admin", time.Now().Add(time.Hour))

	tests := []struct {
		name         string
		setupReq     func(*http.Request)
		allowedRoles []string
		wantValid    bool
		wantErr      error
	}{
		{
			name: "valid JWT in header",
			setupReq: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+validJWT)
			},
			allowedRoles: []string{"admin"},
			wantValid:    true,
			wantErr:      nil,
		},
		{
			name: "valid API key in header",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-API-Key", "valid-api-key")
			},
			allowedRoles: []string{"admin"},
			wantValid:    true,
			wantErr:      nil,
		},
		{
			name: "valid JWT in query",
			setupReq: func(r *http.Request) {
				q := r.URL.Query()
				q.Add("token", validJWT)
				r.URL.RawQuery = q.Encode()
			},
			allowedRoles: []string{"admin"},
			wantValid:    true,
			wantErr:      nil,
		},
		{
			name: "no token provided",
			setupReq: func(r *http.Request) {
				// No token added
			},
			allowedRoles: []string{"admin"},
			wantValid:    false,
			wantErr:      ErrNoToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/test", nil)
			tt.setupReq(req)

			valid, err := svc.ValidateToken(req, tt.allowedRoles)
			assert.Equal(t, tt.wantValid, valid)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
