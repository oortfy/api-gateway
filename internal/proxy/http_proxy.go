package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/util"
	"api-gateway/pkg/discoverer/etcd_discovery"
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
	if route.LoadBalancing != nil {
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
				logger.String("driver", route.LoadBalancing.Driver),
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

			// Extract the real client IP
			clientIP := util.GetClientIP(req)
			p.log.Debug("Extracted client IP for HTTP proxy",
				logger.String("remote_addr", req.RemoteAddr),
				logger.String("client_ip", clientIP),
				logger.String("xff_header", req.Header.Get("X-Forwarded-For")),
				logger.String("xrip_header", req.Header.Get("X-Real-IP")),
			)

			// Add X-Forwarded headers to preserve client IP information
			if clientIP != "" {
				// For X-Forwarded-For, append our client IP to existing chain if present
				if xffHeader := req.Header.Get("X-Forwarded-For"); xffHeader != "" {
					// Keep existing chain and append current client IP to indicate proxy hop
					// But only do this if clientIP is not already the first IP in the chain
					// to avoid duplicating the client IP
					if !strings.HasPrefix(xffHeader, clientIP+",") && xffHeader != clientIP {
						req.Header.Set("X-Forwarded-For", xffHeader)
					}
				} else {
					// No existing chain, just set the client IP
					req.Header.Set("X-Forwarded-For", clientIP)
					p.log.Debug("Set X-Forwarded-For header", logger.String("value", clientIP))
				}

				// Always set X-Real-IP to the original client IP
				req.Header.Set("X-Real-IP", clientIP)
				p.log.Debug("Set X-Real-IP header", logger.String("value", clientIP))
			}

			// Try to resolve country from IP if possible
			country := util.GetGeoLocation(clientIP, p.log)
			if country != "" {
				req.Header.Set("X-Client-Geo-Country", country)
				p.log.Debug("Set X-Client-Geo-Country header",
					logger.String("ip", clientIP),
					logger.String("country", country))
			} else {
				p.log.Debug("No country information available for IP",
					logger.String("ip", clientIP))
			}

			// Check for token in URL query parameters and add it to the headers if present
			// This ensures backward compatibility with clients that send tokens in URL
			token := req.URL.Query().Get("token")
			if token != "" && req.Header.Get("Authorization") == "" {
				req.Header.Set("Authorization", "Bearer "+token)
				p.log.Debug("Added token from URL query to Authorization header")
			}

			// Check for API key in query parameters
			apiKey := req.URL.Query().Get("api_key")
			if apiKey == "" {
				apiKey = req.URL.Query().Get("key")
			}
			if apiKey != "" && req.Header.Get("x-api-key") == "" {
				req.Header.Set("x-api-key", apiKey)
				p.log.Debug("Added API key from URL query to x-api-key header")
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
			driver := loadBalancer.GetDriver()
			if driver != "static" {
				discoveriesConfig := loadBalancer.GetServiceDiscoveries()
				if discoveriesConfig != nil {
					switch driver {
					case "etcd":
						var endpoints = []string{p.config.Etcd.Hosts}
						serviceDiscovery, err := etcd_discovery.NewServiceDiscovery(endpoints, 5*time.Second)
						if err != nil {
							p.log.Error("Connect to etcd error",
								logger.String("etcd", p.config.Etcd.Hosts),
								logger.Error(err),
							)
						} else {
							address, err := serviceDiscovery.DiscoverServices(discoveriesConfig.Prefix, discoveriesConfig.Name) // todo Retrieve the number of failed service retries based on discoveriesConfig.FailLimit
							if err != nil {
								p.log.Error("Failed to discover services",
									logger.String("serviceName", discoveriesConfig.Name),
									logger.Error(err),
								)
							} else {
								healthyEndpoints, err := p.parseURLs("http", address) // The use of HTTP protocol in LAN is faster than HTTPS protocol
								if err == nil {
									loadBalancer.SetHealthyEndpoints(healthyEndpoints)
								} else {
									p.log.Error("Failed to convert address to urls",
										logger.Error(err),
									)
								}
							}
						}
					}
				}
			}

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
	if route.Middlewares.CircuitBreaker != nil && route.Middlewares.CircuitBreaker.Enabled {
		// Create circuit breaker key - unique per route
		circuitKey := route.Path

		// Get or create circuit breaker for this route
		cb, exists := p.circuitBreakers[circuitKey]
		if !exists {
			// Create circuit breaker config
			cbConfig := CircuitBreakerConfig{
				Threshold:     route.Middlewares.CircuitBreaker.Threshold,
				Timeout:       time.Duration(route.Middlewares.CircuitBreaker.Timeout) * time.Second,
				MaxConcurrent: route.Middlewares.CircuitBreaker.MaxConcurrent,
			}

			// Create a new circuit breaker
			cb = NewCircuitBreaker(circuitKey, cbConfig, p.log)
			p.circuitBreakers[circuitKey] = cb

			p.log.Info("Created circuit breaker for route",
				logger.String("path", route.Path),
				logger.Int("threshold", route.Middlewares.CircuitBreaker.Threshold),
				logger.Int("timeout", route.Middlewares.CircuitBreaker.Timeout),
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

// parseURLs returns parsed URL list with protocol auto-completion, or error on invalid format
func (p *HTTPProxy) parseURLs(protocol string, address []string) ([]*url.URL, error) {
	var urls []*url.URL
	for _, addr := range address {
		if !strings.Contains(addr, "://") {
			addr = protocol + "://" + addr
		}

		u, err := url.Parse(addr)
		if err != nil {
			p.log.Error("Invalid URL",
				logger.String("addr", fmt.Sprintf("invalid URL %q", addr)),
				logger.Error(err),
			)
			return nil, fmt.Errorf("invalid URL %q: %w", addr, err)
		}
		urls = append(urls, u)
	}
	return urls, nil
}
