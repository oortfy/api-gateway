package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"api-gateway/internal/config"
)

func TestNewRouter(t *testing.T) {
	// Create a config
	cfg := &config.Config{}

	// Create a logger
	log := &testLogger{}

	// Create a router
	router := NewRouter(cfg, log)

	// Assert the router was created successfully
	assert.NotNil(t, router)
	assert.Equal(t, cfg, router.config)
	assert.NotNil(t, router.router)
	assert.Equal(t, log, router.logger)
}

func TestSetupRoutes(t *testing.T) {
	// Create a config
	cfg := &config.Config{}

	// Create a logger
	log := &testLogger{}

	// Create a router
	router := NewRouter(cfg, log)

	// Create route configurations
	routeConfig := &config.RouteConfig{
		Routes: []config.Route{
			{
				Path:     "/test",
				Protocol: config.ProtocolHTTP,
				Upstream: "http://localhost:8080",
				Methods:  []string{"GET"},
			},
			{
				Path:     "/test-with-middlewares",
				Protocol: config.ProtocolHTTP,
				Upstream: "http://localhost:8080",
				Methods:  []string{"GET"},
				Middlewares: &config.Middlewares{
					RequireAuth: true,
				},
			},
			// This route should be skipped since it's gRPC
			{
				Path:              "test.service.TestService/*",
				Protocol:          config.ProtocolGRPC,
				EndpointsProtocol: config.ProtocolGRPC,
				RPCServer:         "/api/test",
				Upstream:          "grpc://localhost:50051",
			},
		},
	}

	// Setup the routes
	err := router.SetupRoutes(routeConfig)

	// Assert no error occurred
	assert.NoError(t, err)
}

func TestSetupRoutesWithInvalidRoute(t *testing.T) {
	// Create a config
	cfg := &config.Config{}

	// Create a logger
	log := &testLogger{}

	// Create a router
	router := NewRouter(cfg, log)

	// Create route configurations with an invalid route
	routeConfig := &config.RouteConfig{
		Routes: []config.Route{
			{
				// Invalid route: missing path
				Protocol: config.ProtocolHTTP,
				Upstream: "http://localhost:8080",
			},
		},
	}

	// Setup the routes
	err := router.SetupRoutes(routeConfig)

	// Assert an error occurred
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to setup route")
}

func TestServeHTTP(t *testing.T) {
	// Create a config
	cfg := &config.Config{}

	// Create a logger
	log := &testLogger{}

	// Create a router
	router := NewRouter(cfg, log)

	// Create a test request
	req, err := http.NewRequest("GET", "/test", nil)
	require.NoError(t, err)

	// Create a recorder to capture the response
	rr := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(rr, req)

	// Since we haven't set up any routes, this should return 404
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
