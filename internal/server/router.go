package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"api-gateway/internal/config"
	"api-gateway/internal/handlers"
)

// Router handles route configuration and setup
type Router struct {
	config *config.Config
	router *mux.Router
}

// NewRouter creates a new router instance
func NewRouter(cfg *config.Config) *Router {
	return &Router{
		config: cfg,
		router: mux.NewRouter(),
	}
}

// SetupRoutes configures all routes from the configuration
func (r *Router) SetupRoutes(routeConfig *config.RouteConfig) error {
	for _, route := range routeConfig.Routes {
		if err := r.setupRoute(route); err != nil {
			return fmt.Errorf("failed to setup route %s: %w", route.Path, err)
		}
	}
	return nil
}

// setupRoute configures a single route
func (r *Router) setupRoute(route config.Route) error {
	var handler http.Handler

	// Validate route configuration
	if err := route.Validate(); err != nil {
		return fmt.Errorf("invalid route configuration: %w", err)
	}

	// Create appropriate handler based on protocol
	switch route.Protocol {
	case config.ProtocolGRPC:
		grpcHandler, err := handlers.NewGRPCHandler(&route)
		if err != nil {
			return fmt.Errorf("failed to create gRPC handler: %w", err)
		}
		handler = grpcHandler

	case config.ProtocolHTTP:
		httpHandler, err := handlers.NewHTTPHandler(&route)
		if err != nil {
			return fmt.Errorf("failed to create HTTP handler: %w", err)
		}
		handler = httpHandler

	default:
		// This should never happen due to validation
		return fmt.Errorf("unsupported protocol: %s", route.Protocol)
	}

	// Apply middlewares in the correct order
	if route.Middlewares != nil {
		handler = r.applyMiddlewares(handler, route)
	}

	// Register route with the appropriate matching rules
	router := r.router.NewRoute()

	// Add path matching
	if strings.HasSuffix(route.Path, "/*") {
		// For wildcard paths, use PathPrefix
		router.PathPrefix(strings.TrimSuffix(route.Path, "/*"))
	} else {
		// For exact paths
		router.Path(route.Path)
	}

	// Add method restrictions for HTTP routes
	if route.Protocol == config.ProtocolHTTP && len(route.Methods) > 0 {
		router.Methods(route.Methods...)
	}

	// Set the handler
	router.Handler(handler)

	return nil
}

// applyMiddlewares applies configured middlewares to the handler
func (r *Router) applyMiddlewares(handler http.Handler, route config.Route) http.Handler {
	// Apply authentication middleware if required
	if route.Middlewares.RequireAuth {
		handler = handlers.NewAuthMiddleware(handler, r.config.Auth)
	}

	// Apply rate limiting if configured
	if route.Middlewares.RateLimit != nil {
		handler = handlers.NewRateLimitMiddleware(handler, route.Middlewares.RateLimit)
	}

	// Apply circuit breaker if configured
	if route.Middlewares.CircuitBreaker != nil && route.Middlewares.CircuitBreaker.Enabled {
		handler = handlers.NewCircuitBreakerMiddleware(handler, route.Middlewares.CircuitBreaker)
	}

	// Apply caching if configured
	if route.Middlewares.Cache != nil && route.Middlewares.Cache.Enabled {
		handler = handlers.NewCacheMiddleware(handler, route.Middlewares.Cache)
	}

	// Apply compression if enabled
	if route.Compression {
		handler = handlers.NewCompressionMiddleware(handler)
	}

	// Apply CORS if configured
	if r.config.Cors.Enabled {
		handler = handlers.NewCORSMiddleware(handler, r.config.Cors)
	}

	return handler
}

// ServeHTTP implements the http.Handler interface
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}
