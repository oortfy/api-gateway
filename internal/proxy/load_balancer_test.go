package proxy

import (
	"api-gateway/internal/config"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoadBalancer(t *testing.T) {
	log := &mockLogger{}

	t.Run("with valid endpoints", func(t *testing.T) {
		cfg := &config.LoadBalancingConfig{
			Method:      "round_robin",
			Driver:      "static",
			HealthCheck: false,
			Endpoints:   []string{"http://localhost:8001", "http://localhost:8002"},
		}

		lb, err := NewLoadBalancer(cfg, log)
		require.NoError(t, err)
		require.NotNil(t, lb)

		assert.Equal(t, cfg, lb.config)
		assert.Len(t, lb.endpoints, 2)
		assert.Equal(t, uint64(0), lb.counter)
		assert.NotNil(t, lb.healthMap)
		assert.Equal(t, log, lb.log)

		// Verify endpoints were parsed correctly
		assert.Equal(t, "http://localhost:8001", lb.endpoints[0].String())
		assert.Equal(t, "http://localhost:8002", lb.endpoints[1].String())

		// Verify all endpoints are initially marked as healthy
		assert.True(t, lb.healthMap[lb.endpoints[0].String()])
		assert.True(t, lb.healthMap[lb.endpoints[1].String()])
	})

	t.Run("with invalid endpoint", func(t *testing.T) {
		cfg := &config.LoadBalancingConfig{
			Method:      "round_robin",
			Driver:      "static",
			HealthCheck: false,
			Endpoints:   []string{"http://localhost:8001", "://invalid-url"},
		}

		lb, err := NewLoadBalancer(cfg, log)
		require.NoError(t, err)
		require.NotNil(t, lb)

		// Should only have one valid endpoint
		assert.Len(t, lb.endpoints, 1)
		assert.Equal(t, "http://localhost:8001", lb.endpoints[0].String())
	})

	t.Run("with no endpoints", func(t *testing.T) {
		cfg := &config.LoadBalancingConfig{
			Method:      "round_robin",
			Driver:      "static",
			HealthCheck: false,
			Endpoints:   []string{},
		}

		lb, err := NewLoadBalancer(cfg, log)
		assert.Nil(t, lb)
		assert.Nil(t, err)
	})

	t.Run("with nil config", func(t *testing.T) {
		lb, err := NewLoadBalancer(nil, log)
		assert.Nil(t, lb)
		assert.Nil(t, err)
	})
}

func TestGetEndpoint(t *testing.T) {
	log := &mockLogger{}

	t.Run("round_robin_strategy", func(t *testing.T) {
		cfg := &config.LoadBalancingConfig{
			Method:      "round_robin",
			Driver:      "static",
			HealthCheck: false,
			Endpoints:   []string{"http://localhost:8001", "http://localhost:8002", "http://localhost:8003"},
		}

		lb, err := NewLoadBalancer(cfg, log)
		require.NoError(t, err)
		require.NotNil(t, lb)

		// Test that we get different endpoints when calling multiple times
		// and that we eventually cycle through all endpoints
		seenEndpoints := make(map[string]bool)

		// Should see all 3 endpoints after at most 10 calls
		maxCalls := 10
		for i := 0; i < maxCalls; i++ {
			endpoint := lb.GetEndpoint()
			seenEndpoints[endpoint.String()] = true

			// If we've seen all endpoints, we're done
			if len(seenEndpoints) == 3 {
				break
			}
		}

		// Should have seen all 3 endpoints
		assert.Equal(t, 3, len(seenEndpoints), "Should have seen all 3 endpoints")
		assert.True(t, seenEndpoints["http://localhost:8001"], "Should have seen endpoint 8001")
		assert.True(t, seenEndpoints["http://localhost:8002"], "Should have seen endpoint 8002")
		assert.True(t, seenEndpoints["http://localhost:8003"], "Should have seen endpoint 8003")
	})

	t.Run("random_strategy", func(t *testing.T) {
		cfg := &config.LoadBalancingConfig{
			Method:      "random",
			Driver:      "static",
			HealthCheck: false,
			Endpoints:   []string{"http://localhost:8001", "http://localhost:8002", "http://localhost:8003"},
		}

		lb, err := NewLoadBalancer(cfg, log)
		require.NoError(t, err)
		require.NotNil(t, lb)

		// Make multiple calls and verify we get only valid endpoints
		validEndpoints := map[string]bool{
			"http://localhost:8001": true,
			"http://localhost:8002": true,
			"http://localhost:8003": true,
		}

		for i := 0; i < 10; i++ {
			endpoint := lb.GetEndpoint()
			assert.True(t, validEndpoints[endpoint.String()], "Endpoint should be from the valid list")
		}
	})

	t.Run("with_unhealthy_endpoints", func(t *testing.T) {
		cfg := &config.LoadBalancingConfig{
			Method:      "round_robin",
			Driver:      "static",
			HealthCheck: false,
			Endpoints:   []string{"http://localhost:8001", "http://localhost:8002"},
		}

		lb, err := NewLoadBalancer(cfg, log)
		require.NoError(t, err)
		require.NotNil(t, lb)

		// Mark the first endpoint as unhealthy
		lb.healthLock.Lock()
		lb.healthMap[lb.endpoints[0].String()] = false
		lb.healthLock.Unlock()

		// Should return the healthy endpoint
		endpoint := lb.GetEndpoint()
		assert.Equal(t, "http://localhost:8002", endpoint.String())

		// Mark all endpoints as unhealthy
		lb.healthLock.Lock()
		lb.healthMap[lb.endpoints[1].String()] = false
		lb.healthLock.Unlock()

		// Should return any endpoint when none are healthy
		endpoint = lb.GetEndpoint()
		assert.NotNil(t, endpoint)
	})
}

func TestHealthCheck(t *testing.T) {
	// Create test servers to act as healthy and unhealthy endpoints
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthyServer.Close()

	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer unhealthyServer.Close()

	// Parse URLs for direct testing
	healthyURL, _ := url.Parse(healthyServer.URL)
	unhealthyURL, _ := url.Parse(unhealthyServer.URL)

	log := &mockLogger{}
	cfg := &config.LoadBalancingConfig{
		Method:      "round_robin",
		Driver:      "static",
		HealthCheck: true,
		Endpoints:   []string{healthyServer.URL, unhealthyServer.URL},
		HealthCheckConfig: &config.HealthCheckConfig{
			Path:     "/",
			Interval: 1,
			Timeout:  1,
		},
	}

	lb, err := NewLoadBalancer(cfg, log)
	require.NoError(t, err)
	require.NotNil(t, lb)

	// Manually trigger health checks
	lb.checkEndpointHealth(healthyURL)
	lb.checkEndpointHealth(unhealthyURL)

	// Verify health status
	lb.healthLock.RLock()
	healthyStatus := lb.healthMap[healthyURL.String()]
	unhealthyStatus := lb.healthMap[unhealthyURL.String()]
	lb.healthLock.RUnlock()

	assert.True(t, healthyStatus, "Healthy endpoint should be marked as healthy")
	assert.False(t, unhealthyStatus, "Unhealthy endpoint should be marked as unhealthy")

	// Test healthy endpoints list
	healthyEndpoints := lb.getHealthyEndpoints()
	assert.Len(t, healthyEndpoints, 1)
	assert.Equal(t, healthyURL.String(), healthyEndpoints[0].String())
}

func TestGetDriver(t *testing.T) {
	log := &mockLogger{}
	cfg := &config.LoadBalancingConfig{
		Method:    "round_robin",
		Driver:    "static",
		Endpoints: []string{"http://localhost:8001"}, // Need at least one endpoint
	}

	lb, err := NewLoadBalancer(cfg, log)
	require.NoError(t, err)
	require.NotNil(t, lb)

	driver := lb.GetDriver()
	assert.Equal(t, "static", driver)
}

func TestGetServiceDiscoveries(t *testing.T) {
	log := &mockLogger{}
	discoveries := &config.Discoveries{
		Name:      "test",
		Prefix:    "/services",
		FailLimit: 3,
	}

	cfg := &config.LoadBalancingConfig{
		Method:      "round_robin",
		Driver:      "etcd",
		Discoveries: discoveries,
	}

	lb, err := NewLoadBalancer(cfg, log)
	require.NoError(t, err)
	require.NotNil(t, lb)

	result := lb.GetServiceDiscoveries()
	assert.Equal(t, discoveries, result)
}

// TestLoadBalancer_SetHealthyEndpoints tests the SetHealthyEndpoints method
func TestLoadBalancer_SetHealthyEndpoints(t *testing.T) {
	// Create mock logger
	log := &mockLogger{}

	// Create a load balancer with initial endpoints
	config := &config.LoadBalancingConfig{
		Method:      "round_robin",
		HealthCheck: false,
		Endpoints:   []string{"http://endpoint1.example.com", "http://endpoint2.example.com"},
	}

	lb, err := NewLoadBalancer(config, log)
	require.NoError(t, err)
	require.NotNil(t, lb)

	// Initial endpoints check
	initialEndpoints := lb.endpoints
	require.Equal(t, 2, len(initialEndpoints))

	// Create new endpoints
	newEndpoint1, _ := url.Parse("http://new1.example.com")
	newEndpoint2, _ := url.Parse("http://new2.example.com")
	newEndpoint3, _ := url.Parse("http://new3.example.com")
	newEndpoints := []*url.URL{newEndpoint1, newEndpoint2, newEndpoint3}

	// Set new endpoints
	result := lb.SetHealthyEndpoints(newEndpoints)
	assert.True(t, result)

	// Check that endpoints were updated
	assert.Equal(t, 3, len(lb.endpoints))
	assert.Contains(t, lb.endpoints, newEndpoint1)
	assert.Contains(t, lb.endpoints, newEndpoint2)
	assert.Contains(t, lb.endpoints, newEndpoint3)
}

// TestLoadBalancer_GetDriver tests the GetDriver method
func TestLoadBalancer_GetDriver(t *testing.T) {
	// Create mock logger
	log := &mockLogger{}

	// Test different driver types
	testCases := []struct {
		name       string
		driverType string
	}{
		{
			name:       "static_driver",
			driverType: "static",
		},
		{
			name:       "etcd_driver",
			driverType: "etcd",
		},
		{
			name:       "consul_driver",
			driverType: "consul",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &config.LoadBalancingConfig{
				Method:      "round_robin",
				Driver:      tc.driverType,
				HealthCheck: false,
				Endpoints:   []string{"http://endpoint.example.com"},
			}

			lb, err := NewLoadBalancer(config, log)
			require.NoError(t, err)
			require.NotNil(t, lb)

			driver := lb.GetDriver()
			assert.Equal(t, tc.driverType, driver)
		})
	}
}

// TestLoadBalancer_GetServiceDiscoveries tests the GetServiceDiscoveries method
func TestLoadBalancer_GetServiceDiscoveries(t *testing.T) {
	// Create mock logger
	log := &mockLogger{}

	// Create a discoveries configuration
	discoveries := &config.Discoveries{
		Name:      "test-service",
		Prefix:    "/services/",
		FailLimit: 3,
	}

	// Create load balancer config with discoveries
	config := &config.LoadBalancingConfig{
		Method:      "round_robin",
		Driver:      "etcd",
		HealthCheck: false,
		Endpoints:   []string{"http://endpoint.example.com"},
		Discoveries: discoveries,
	}

	lb, err := NewLoadBalancer(config, log)
	require.NoError(t, err)
	require.NotNil(t, lb)

	// Get the discoveries config
	result := lb.GetServiceDiscoveries()
	assert.Equal(t, discoveries, result)
	assert.Equal(t, "test-service", result.Name)
	assert.Equal(t, "/services/", result.Prefix)
	assert.Equal(t, 3, result.FailLimit)
}

// TestLoadBalancer_CheckEndpointHealth tests the checkEndpointHealth method
func TestLoadBalancer_CheckEndpointHealth(t *testing.T) {
	// Create a test server that simulates health checks
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer healthyServer.Close()

	// Create a test server that simulates unhealthy responses
	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer unhealthyServer.Close()

	// Create a mock logger to capture logs
	log := &mockLogger{}

	// Create a custom health check config
	healthCheckConfig := &config.HealthCheckConfig{
		Path:               "/health",
		Interval:           1,
		Timeout:            1,
		HealthyThreshold:   2,
		UnhealthyThreshold: 2,
	}

	// Create a load balancer with both healthy and unhealthy endpoints
	healthyURL, _ := url.Parse(healthyServer.URL)
	unhealthyURL, _ := url.Parse(unhealthyServer.URL)
	invalidURL, _ := url.Parse("http://invalid-host.local:12345")

	config := &config.LoadBalancingConfig{
		Method:            "round_robin",
		HealthCheck:       true,
		HealthCheckConfig: healthCheckConfig,
	}

	lb := &LoadBalancer{
		config:    config,
		endpoints: []*url.URL{healthyURL, unhealthyURL, invalidURL},
		healthMap: make(map[string]bool),
		log:       log,
	}

	// Test the health check on the healthy endpoint
	lb.checkEndpointHealth(healthyURL)
	assert.True(t, lb.healthMap[healthyURL.String()])

	// Test the health check on the unhealthy endpoint
	lb.checkEndpointHealth(unhealthyURL)
	assert.False(t, lb.healthMap[unhealthyURL.String()])

	// Test the health check on an invalid endpoint (connection error)
	lb.checkEndpointHealth(invalidURL)
	assert.False(t, lb.healthMap[invalidURL.String()])
}

// TestLoadBalancer_GetErrorMessage tests the getErrorMessage function
func TestLoadBalancer_GetErrorMessage(t *testing.T) {
	// Test with nil error
	msg := getErrorMessage(nil)
	assert.Equal(t, "unknown error", msg)

	// Test with actual error
	err := fmt.Errorf("test error")
	msg = getErrorMessage(err)
	assert.Equal(t, "test error", msg)
}
