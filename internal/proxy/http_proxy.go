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
}

// NewHTTPProxy creates a new HTTP proxy
func NewHTTPProxy(config *config.Config, routes *config.RouteConfig, log logger.Logger) *HTTPProxy {
	return &HTTPProxy{
		config: config,
		routes: routes,
		log:    log,
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

	// Create a reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Customize the director function to modify the request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		// Call the original director
		originalDirector(req)

		// Modify the request URL
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host

		// Handle path stripping if enabled
		if route.StripPrefix && strings.HasPrefix(req.URL.Path, route.Path) {
			// Remove the route path prefix from the URL
			req.URL.Path = strings.TrimPrefix(req.URL.Path, route.Path)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}

		// Update the Host header to match the target
		req.Host = target.Host

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
			logger.String("upstream", target.String()),
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the request
		p.log.Debug("Proxying request",
			logger.String("path", r.URL.Path),
			logger.String("method", r.Method),
			logger.String("upstream", target.String()),
		)

		// Proxy the request to the upstream service
		proxy.ServeHTTP(w, r)
	})
}
