package proxy

import (
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
)

// LoadBalancer provides load balancing functionality
type LoadBalancer struct {
	config     *config.LoadBalancingConfig
	endpoints  []*url.URL
	counter    uint64
	healthMap  map[string]bool
	healthLock sync.RWMutex
	log        logger.Logger
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(config *config.LoadBalancingConfig, log logger.Logger) (*LoadBalancer, error) {
	if config == nil || len(config.Endpoints) == 0 {
		return nil, nil
	}

	var endpoints []*url.URL
	for _, endpoint := range config.Endpoints {
		url, err := url.Parse(endpoint)
		if err != nil {
			log.Error("Failed to parse load balancer endpoint",
				logger.String("endpoint", endpoint),
				logger.Error(err),
			)
			continue
		}
		endpoints = append(endpoints, url)
	}

	if len(endpoints) == 0 {
		return nil, nil
	}

	lb := &LoadBalancer{
		config:    config,
		endpoints: endpoints,
		counter:   0,
		healthMap: make(map[string]bool),
		log:       log,
	}

	// Initialize all endpoints as healthy
	for _, endpoint := range endpoints {
		lb.healthMap[endpoint.String()] = true
	}

	// Start health checking if enabled
	if config.HealthCheck {
		go lb.startHealthCheck()
	}

	return lb, nil
}

// GetEndpoint returns the next endpoint based on the load balancing strategy
func (lb *LoadBalancer) GetEndpoint() *url.URL {
	// First check if we have any healthy endpoints
	healthyEndpoints := lb.getHealthyEndpoints()
	if len(healthyEndpoints) == 0 {
		// If no healthy endpoints, return any endpoint (better than nothing)
		return lb.getAnyEndpoint()
	}

	// Select endpoint based on strategy
	switch lb.config.Method {
	case "random":
		return lb.getRandomEndpoint(healthyEndpoints)
	case "round_robin":
		return lb.getRoundRobinEndpoint(healthyEndpoints)
	default:
		// Default to round-robin
		return lb.getRoundRobinEndpoint(healthyEndpoints)
	}
}

// getHealthyEndpoints returns only the healthy endpoints
func (lb *LoadBalancer) getHealthyEndpoints() []*url.URL {
	lb.healthLock.RLock()
	defer lb.healthLock.RUnlock()

	var healthy []*url.URL
	for _, endpoint := range lb.endpoints {
		if lb.healthMap[endpoint.String()] {
			healthy = append(healthy, endpoint)
		}
	}
	return healthy
}

// getAnyEndpoint returns any endpoint regardless of health status
func (lb *LoadBalancer) getAnyEndpoint() *url.URL {
	// Just use round-robin on all endpoints
	count := atomic.AddUint64(&lb.counter, 1)
	return lb.endpoints[count%uint64(len(lb.endpoints))]
}

// getRandomEndpoint returns a random endpoint from the given list
func (lb *LoadBalancer) getRandomEndpoint(endpoints []*url.URL) *url.URL {
	return endpoints[rand.Intn(len(endpoints))]
}

// getRoundRobinEndpoint returns the next endpoint in round-robin fashion
func (lb *LoadBalancer) getRoundRobinEndpoint(endpoints []*url.URL) *url.URL {
	count := atomic.AddUint64(&lb.counter, 1)
	return endpoints[count%uint64(len(endpoints))]
}

// startHealthCheck periodically checks the health of all endpoints
func (lb *LoadBalancer) startHealthCheck() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		lb.checkEndpointsHealth()
	}
}

// checkEndpointsHealth checks the health of all endpoints
func (lb *LoadBalancer) checkEndpointsHealth() {
	for _, endpoint := range lb.endpoints {
		go lb.checkEndpointHealth(endpoint)
	}
}

// checkEndpointHealth checks the health of a single endpoint
func (lb *LoadBalancer) checkEndpointHealth(endpoint *url.URL) {
	// Create a health check URL (often /health or /status)
	healthURL := *endpoint
	healthURL.Path = "/health"

	// Create a client with short timeout
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	// Make the request
	resp, err := client.Get(healthURL.String())

	// Update health status
	lb.healthLock.Lock()
	defer lb.healthLock.Unlock()

	// Mark as healthy if no error and status is 2xx
	isHealthy := err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300

	// Only log if status changes
	if lb.healthMap[endpoint.String()] != isHealthy {
		if isHealthy {
			lb.log.Info("Endpoint is now healthy",
				logger.String("endpoint", endpoint.String()),
			)
		} else {
			lb.log.Warn("Endpoint is unhealthy",
				logger.String("endpoint", endpoint.String()),
				logger.Error(err),
			)
		}
	}

	lb.healthMap[endpoint.String()] = isHealthy

	// Close response body if not nil
	if resp != nil {
		resp.Body.Close()
	}
}
