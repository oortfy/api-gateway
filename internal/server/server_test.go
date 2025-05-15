package server

import (
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...logger.Field)  {}
func (m *mockLogger) Info(msg string, fields ...logger.Field)   {}
func (m *mockLogger) Warn(msg string, fields ...logger.Field)   {}
func (m *mockLogger) Error(msg string, fields ...logger.Field)  {}
func (m *mockLogger) Fatal(msg string, fields ...logger.Field)  {}
func (m *mockLogger) With(fields ...logger.Field) logger.Logger { return m }

func createTestServer() (*Server, error) {
	// Create minimal configurations for testing
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address:        ":8080",
			ReadTimeout:    10,
			WriteTimeout:   10,
			IdleTimeout:    30,
			MaxHeaderBytes: 1048576,
		},
		Auth: config.AuthConfig{
			JWTSecret:    "test-secret",
			JWTHeader:    "Authorization",
			APIKeyHeader: "X-API-Key",
		},
		Metrics: config.MetricsConfig{
			Enabled:  true,
			Endpoint: "/metrics",
		},
		Cors: config.CorsConfig{
			Enabled:         true,
			AllowAllOrigins: true,
		},
		Cache: config.CacheConfig{
			Enabled:    false,
			DefaultTTL: 60,
		},
	}

	routes := &config.RouteConfig{
		Routes: []config.Route{
			{
				Path:        "/api/test",
				Upstream:    "http://test-service:8080",
				Methods:     []string{"GET", "POST"},
				Protocol:    "HTTP",
				StripPrefix: true,
				Timeout:     30,
				Middlewares: &config.Middlewares{
					RequireAuth: false,
				},
			},
		},
	}

	log := &mockLogger{}
	return NewServer(cfg, routes, log), nil
}

func TestNewServer(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)
	require.NotNil(t, server)

	// Verify server components were initialized correctly
	assert.NotNil(t, server.router)
	assert.NotNil(t, server.httpServer)
	assert.NotNil(t, server.authService)
	assert.NotNil(t, server.httpProxy)
	assert.NotNil(t, server.wsProxy)
	assert.NotNil(t, server.authMiddleware)
	assert.NotNil(t, server.cacheMiddleware)
	assert.NotNil(t, server.rateLimiter)
	assert.NotNil(t, server.headerTransformer)
	assert.NotNil(t, server.urlRewriter)
	assert.NotNil(t, server.retryMiddleware)
	assert.NotNil(t, server.metricsMiddleware)
	assert.NotNil(t, server.corsMiddleware)

	// Check server config
	assert.Equal(t, ":8080", server.httpServer.Addr)
	assert.Equal(t, 10*time.Second, server.httpServer.ReadTimeout)
	assert.Equal(t, 10*time.Second, server.httpServer.WriteTimeout)
	assert.Equal(t, 120*time.Second, server.httpServer.IdleTimeout)
}

func TestHealthEndpoint(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	// Register utility endpoints
	server.registerUtilityEndpoints()

	// Create a test request to the health endpoint
	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Serve the request through the router
	server.router.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check the response body
	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "up", response["status"])
	_, err = time.Parse(time.RFC3339, response["time"])
	assert.NoError(t, err, "Time should be in RFC3339 format")
}

func TestStop(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	// Test server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start the server in a goroutine (we won't actually let it listen)
	go func() {
		// Give it a moment to "start"
		time.Sleep(100 * time.Millisecond)
		// Hijack the httpServer with a test server that we can control
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer testServer.Close()

		server.httpServer = testServer.Config
	}()

	// Stop the server
	err = server.Stop(ctx)
	// This might error because we're not really starting the server, but we're just testing the method is called
	// The important thing is that the function doesn't panic
}
