package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
)

// HTTPProxy handles HTTP requests to upstream services
type HTTPProxy struct {
	config *config.Config
	routes *config.RouteConfig
	log    logger.Logger
	// Map to store circuit breakers for routes
	circuitBreakers map[string]*CircuitBreaker
}

// NewHTTPProxy creates a new HTTP proxy
func NewHTTPProxy(config *config.Config, routes *config.RouteConfig, log logger.Logger) *HTTPProxy {
	return &HTTPProxy{
		config:          config,
		routes:          routes,
		log:             log,
		circuitBreakers: make(map[string]*CircuitBreaker),
	}
}

// ProxyRequest forwards the request to the upstream service
func (p *HTTPProxy) ProxyRequest(route config.Route) http.Handler {
	// Parse the upstream URL
	target, err := url.Parse(route.Upstream)
	if err != nil {
		p.log.Error("Failed to parse upstream URL",
			logger.String("upstream", route.Upstream),
			logger.Error(err),
		)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		})
	}

	// Create load balancer if configured
	var loadBalancer *LoadBalancer
	if route.LoadBalancing != nil && len(route.LoadBalancing.Endpoints) > 0 {
		loadBalancer, err = NewLoadBalancer(route.LoadBalancing, p.log)
		if err != nil {
			p.log.Error("Failed to create load balancer",
				logger.String("path", route.Path),
				logger.Error(err),
			)
		} else {
			p.log.Info("Created load balancer for route",
				logger.String("path", route.Path),
				logger.String("method", route.LoadBalancing.Method),
				logger.Int("endpoints", len(route.LoadBalancing.Endpoints)),
			)
		}
	}

	// Create a proxy handler factory function that can select the target
	createProxy := func(targetURL *url.URL) *httputil.ReverseProxy {
		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		// Customize the director function to modify the request
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			// Call the original director
			originalDirector(req)

			// Modify the request URL
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host

			// Handle path stripping if enabled
			if route.StripPrefix && strings.HasPrefix(req.URL.Path, route.Path) {
				// Remove the route path prefix from the URL
				req.URL.Path = strings.TrimPrefix(req.URL.Path, route.Path)
				if req.URL.Path == "" {
					req.URL.Path = "/"
				}
			}

			// Update the Host header to match the target
			req.Host = targetURL.Host

			// Add X-Forwarded headers
			if _, ok := req.Header["X-Forwarded-For"]; !ok {
				req.Header.Set("X-Forwarded-For", req.RemoteAddr)
			}
			req.Header.Set("X-Forwarded-Host", req.Host)
			req.Header.Set("X-Forwarded-Proto", req.URL.Scheme)
			req.Header.Set("X-Gateway-Proxy", "true")
		}

		// Customize the error handler
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			p.log.Error("Proxy error",
				logger.String("path", r.URL.Path),
				logger.String("method", r.Method),
				logger.String("upstream", targetURL.String()),
				logger.Error(err),
			)
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		}

		// Set timeouts
		if route.Timeout > 0 {
			timeout := time.Duration(route.Timeout) * time.Second
			proxy.Transport = &http.Transport{
				ResponseHeaderTimeout: timeout,
				ExpectContinueTimeout: 1 * time.Second,
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   100,
				IdleConnTimeout:       90 * time.Second,
			}
		}

		return proxy
	}

	// Create the final handler
	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Select target - either from load balancer or static
		targetURL := target
		if loadBalancer != nil {
			targetURL = loadBalancer.GetEndpoint()
			p.log.Debug("Using load balanced endpoint",
				logger.String("path", r.URL.Path),
				logger.String("endpoint", targetURL.String()),
			)
		}

		// Create or get proxy for this target
		proxy := createProxy(targetURL)

		// Log the request
		p.log.Debug("Proxying request",
			logger.String("path", r.URL.Path),
			logger.String("method", r.Method),
			logger.String("upstream", targetURL.String()),
		)

		// Proxy the request to the upstream service
		proxy.ServeHTTP(w, r)
	})

	// Apply circuit breaker if enabled
	if route.CircuitBreaker != nil && route.CircuitBreaker.Enabled {
		// Create circuit breaker key - unique per route
		circuitKey := route.Path

		// Get or create circuit breaker for this route
		cb, exists := p.circuitBreakers[circuitKey]
		if !exists {
			// Create circuit breaker config
			cbConfig := CircuitBreakerConfig{
				Threshold:     route.CircuitBreaker.Threshold,
				Timeout:       time.Duration(route.CircuitBreaker.Timeout) * time.Second,
				MaxConcurrent: route.CircuitBreaker.MaxConcurrent,
			}

			// Create a new circuit breaker
			cb = NewCircuitBreaker(circuitKey, cbConfig, p.log)
			p.circuitBreakers[circuitKey] = cb

			p.log.Info("Created circuit breaker for route",
				logger.String("path", route.Path),
				logger.Int("threshold", route.CircuitBreaker.Threshold),
				logger.Int("timeout", route.CircuitBreaker.Timeout),
			)
		}

		// Wrap the proxy handler with circuit breaker
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Execute the request through the circuit breaker
			if err := cb.Execute(r, proxyHandler, w); err != nil {
				// Error is already handled inside the Execute method
				return
			}
		})
	}

	// Return the standard proxy handler if circuit breaker is not enabled
	return proxyHandler
}
