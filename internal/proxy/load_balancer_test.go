package proxy

import (
	"api-gateway/internal/config"
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
