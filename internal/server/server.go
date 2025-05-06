package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"api-gateway/internal/auth"
	"api-gateway/internal/config"
	"api-gateway/internal/handlers"
	"api-gateway/internal/middleware"
	"api-gateway/internal/proxy"
	"api-gateway/internal/util"
	"api-gateway/pkg/logger"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server represents the API Gateway server
type Server struct {
	config            *config.Config
	routes            *config.RouteConfig
	log               logger.Logger
	httpServer        *http.Server
	router            *mux.Router
	authService       *auth.AuthService
	httpProxy         *proxy.HTTPProxy
	wsProxy           *proxy.WSProxy
	authMiddleware    *middleware.AuthMiddleware
	cacheMiddleware   *middleware.CacheMiddleware
	rateLimiter       *middleware.RateLimiter
	headerTransformer *middleware.HeaderTransformer
	urlRewriter       *middleware.URLRewriter
	retryMiddleware   *middleware.RetryMiddleware
	metricsMiddleware *middleware.MetricsMiddleware
	corsMiddleware    *middleware.CORSMiddleware
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config, routes *config.RouteConfig, log logger.Logger) *Server {
	router := mux.NewRouter()

	// Initialize services
	authService := auth.NewAuthService(&cfg.Auth, log)
	httpProxy := proxy.NewHTTPProxy(cfg, routes, log)
	wsProxy := proxy.NewWSProxy(cfg, routes, log)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(authService, &cfg.Auth, log)
	cacheMiddleware := middleware.NewCacheMiddleware(&cfg.Cache, log)
	rateLimiter := middleware.NewRateLimiter(log)
	headerTransformer := middleware.NewHeaderTransformer(log)
	urlRewriter := middleware.NewURLRewriter(log)
	retryMiddleware := middleware.NewRetryMiddleware(log)
	metricsMiddleware := middleware.NewMetricsMiddleware(&cfg.Metrics, log)

	// Convert CorsConfig to CORSConfig
	corsConfig := &config.CORSConfig{
		Enabled:          cfg.Cors.Enabled,
		AllowAllOrigins:  cfg.Cors.AllowAllOrigins,
		AllowedOrigins:   cfg.Cors.AllowedOrigins,
		AllowedMethods:   cfg.Cors.AllowedMethods,
		AllowedHeaders:   cfg.Cors.AllowedHeaders,
		ExposedHeaders:   cfg.Cors.ExposedHeaders,
		AllowCredentials: cfg.Cors.AllowCredentials,
		MaxAge:           cfg.Cors.MaxAge,
	}
	corsMiddleware := middleware.NewCORSMiddleware(corsConfig, log)

	// Setup rate limiters for routes with rate limiting enabled
	for _, route := range routes.Routes {
		if route.RateLimit != nil && route.RateLimit.Requests > 0 {
			rateLimiter.AddLimit(route.Path, *route.RateLimit)
		}
	}

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Apply global middleware
	// CORS middleware should be first in the chain
	if cfg.Cors.Enabled {
		router.Use(corsMiddleware.CORS)
		log.Info("Applied CORS middleware globally")
	}

	return &Server{
		config:            cfg,
		routes:            routes,
		log:               log,
		httpServer:        httpServer,
		router:            router,
		authService:       authService,
		httpProxy:         httpProxy,
		wsProxy:           wsProxy,
		authMiddleware:    authMiddleware,
		cacheMiddleware:   cacheMiddleware,
		rateLimiter:       rateLimiter,
		headerTransformer: headerTransformer,
		urlRewriter:       urlRewriter,
		retryMiddleware:   retryMiddleware,
		metricsMiddleware: metricsMiddleware,
		corsMiddleware:    corsMiddleware,
	}
}

// Start initializes and starts the server
func (s *Server) Start() error {
	// Register routes
	for _, route := range s.routes.Routes {
		s.registerRoute(route)
	}

	// Register additional utility endpoints
	s.registerUtilityEndpoints()

	// Start the HTTP server
	s.log.Info("Starting API Gateway server",
		logger.String("address", s.config.Server.Address),
	)

	return s.httpServer.ListenAndServe()
}

// registerUtilityEndpoints registers endpoints for health check, metrics, etc.
func (s *Server) registerUtilityEndpoints() {
	// Register health check endpoint
	s.router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "up",
			"time":   time.Now().Format(time.RFC3339),
		})
	}).Methods("GET")

	// Register metrics endpoint if enabled
	if s.config.Metrics.Enabled {
		s.router.Handle(s.config.Metrics.Endpoint, promhttp.Handler())
	}

	// Register Swagger documentation
	s.router.PathPrefix("/docs/swagger/").Handler(http.StripPrefix("/docs/swagger/", http.FileServer(http.Dir("./docs/swagger"))))
	s.log.Info("Registered Swagger documentation endpoint",
		logger.String("path", "/docs/swagger/"),
	)

	// Register test endpoint for client IP detection and token validation
	s.router.HandleFunc("/test-ip", func(w http.ResponseWriter, r *http.Request) {
		clientIP := util.GetClientIP(r)
		country := util.GetGeoLocation(clientIP, s.log)

		// Extract tokens from various sources
		authHeader := r.Header.Get("Authorization")
		apiKeyHeader := r.Header.Get("x-api-key")
		tokenQuery := r.URL.Query().Get("token")
		apiKeyQuery := r.URL.Query().Get("api_key")
		if apiKeyQuery == "" {
			apiKeyQuery = r.URL.Query().Get("key")
		}

		// Format response
		w.Header().Set("Content-Type", "application/json")
		jsonResponse := map[string]interface{}{
			"client_ip":   clientIP,
			"remote_addr": r.RemoteAddr,
			"country":     country,
			"headers": map[string]string{
				"x-forwarded-for": r.Header.Get("X-Forwarded-For"),
				"x-real-ip":       r.Header.Get("X-Real-IP"),
				"authorization":   authHeader,
				"x-api-key":       apiKeyHeader,
			},
			"query_parameters": map[string]string{
				"token":   tokenQuery,
				"api_key": apiKeyQuery,
			},
			"time": time.Now().Format(time.RFC3339),
		}

		// Add auth-specific data
		if authHeader != "" || tokenQuery != "" {
			jsonResponse["auth_method"] = "jwt"
		} else if apiKeyHeader != "" || apiKeyQuery != "" {
			jsonResponse["auth_method"] = "api_key"
		} else {
			jsonResponse["auth_method"] = "none"
		}

		json.NewEncoder(w).Encode(jsonResponse)

		s.log.Info("Received request to /test-ip",
			logger.String("client_ip", clientIP),
			logger.String("country", country),
			logger.String("remote_addr", r.RemoteAddr),
		)
	}).Methods("GET")
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	s.log.Info("Shutting down server...")
	return s.httpServer.Shutdown(ctx)
}

// registerRoutes configures all the route handlers
func (s *Server) registerRoutes() {
	// Add health check endpoint
	s.router.HandleFunc("/health", handlers.HealthCheckHandler).Methods("GET")

	// Register all routes from configuration
	for _, route := range s.routes.Routes {
		s.registerRoute(route)
	}

	// Add catch-all route for 404 responses
	s.router.NotFoundHandler = http.HandlerFunc(handlers.NotFoundHandler)
}

// registerRoute configures an individual route
func (s *Server) registerRoute(route config.Route) {
	// Create a new router for this route
	var routeRouter *mux.Router
	if strings.HasSuffix(route.Path, "/*") {
		route.Path = strings.TrimRight(route.Path, "/*")
		routeRouter = s.router.PathPrefix(route.Path).Subrouter()
	}

	// Register the appropriate handlers based on whether it's a WebSocket route or not
	switch route.Protocol {
	case "SOCKET":
		if route.WebSocket == nil && route.WebSocket.Enabled == false {
			return
		}

		// WebSocket handler
		wsHandler := s.wsProxy.ProxyWebSocket(route)

		// Apply authentication middleware if required
		if route.RequireAuth {
			wsHandler = s.authMiddleware.Authenticate(wsHandler, route)
		}

		// Register the handler for the WebSocket-specific path or the general route path
		wsPath := route.WebSocket.Path
		if wsPath == "" {
			if routeRouter == nil {
				routeRouter = s.router
				routeRouter.PathPrefix("/").Path(route.Path).Handler(wsHandler)
			} else {
				// If no specific path is provided, use the general path
				routeRouter.PathPrefix("/").Handler(wsHandler)
			}
			s.log.Info("Registered WebSocket route",
				logger.String("path", fmt.Sprintf("%s/*", route.Path)),
				logger.String("upstream", route.Upstream),
			)
		} else {
			// Register handler for the specific WebSocket path
			s.router.Path(wsPath).Handler(wsHandler)
			s.log.Info("Registered WebSocket route",
				logger.String("path", wsPath),
				logger.String("upstream", route.Upstream),
			)
		}
	case "HTTP":
		// HTTP handler
		httpHandler := s.httpProxy.ProxyRequest(route)

		// Apply URL rewriting if configured
		if route.URLRewrite != nil && len(route.URLRewrite.Patterns) > 0 {
			httpHandler = s.urlRewriter.Rewrite(httpHandler, route.URLRewrite)
			s.log.Info("Applied URL rewriting to route",
				logger.String("path", route.Path),
				logger.Int("patterns", len(route.URLRewrite.Patterns)),
			)
		}

		// Apply header transformations if configured
		if route.HeaderTransform != nil {
			httpHandler = s.headerTransformer.Transform(httpHandler, route.HeaderTransform)
			s.log.Info("Applied header transformation to route",
				logger.String("path", route.Path),
			)
		}

		// Apply rate limiting if enabled
		if route.RateLimit != nil && route.RateLimit.Requests > 0 {
			httpHandler = s.rateLimiter.RateLimit(httpHandler, route)
			s.log.Info("Applied rate limiting to route",
				logger.String("path", route.Path),
				logger.Int("requests", route.RateLimit.Requests),
				logger.String("period", route.RateLimit.Period),
			)
		}

		// Apply retry policy if enabled
		if route.RetryPolicy != nil && route.RetryPolicy.Enabled {
			httpHandler = s.retryMiddleware.Retry(httpHandler, route.RetryPolicy)
			s.log.Info("Applied retry policy to route",
				logger.String("path", route.Path),
				logger.Int("attempts", route.RetryPolicy.Attempts),
				logger.Int("per_try_timeout", route.RetryPolicy.PerTryTimeout),
			)
		}

		// Apply cache middleware if enabled for this route
		if s.config.Cache.Enabled && route.Cache != nil && route.Cache.Enabled {
			httpHandler = s.cacheMiddleware.Cache(httpHandler, route)
			s.log.Info("Applied cache middleware to route",
				logger.String("path", route.Path),
				logger.Int("ttl", route.Cache.TTL),
				logger.Bool("cache_authenticated", route.Cache.CacheAuthenticated),
			)
		}

		// Apply authentication middleware if required
		if route.RequireAuth {
			httpHandler = s.authMiddleware.Authenticate(httpHandler, route)
		}

		// If methods are specified, register the handler for each method
		if len(route.Methods) > 0 {
			for _, method := range route.Methods {
				if routeRouter == nil {
					routeRouter = s.router
					routeRouter.PathPrefix("/").Path(route.Path).Handler(httpHandler).Methods(method)
				} else {
					routeRouter.PathPrefix("/").Handler(httpHandler).Methods(method)
				}
				s.log.Info("Registered route",
					logger.String("path", fmt.Sprintf("%s/*", route.Path)),
					logger.String("method", method),
					logger.String("upstream", route.Upstream),
				)
			}
		} else {
			if routeRouter == nil {
				routeRouter = s.router
				routeRouter.PathPrefix("/").Path(route.Path).Handler(httpHandler)
			} else {
				// Otherwise, register for all methods
				routeRouter.PathPrefix("/").Handler(httpHandler)
			}
			s.log.Info("Registered route",
				logger.String("path", fmt.Sprintf("%s/*", route.Path)),
				logger.String("method", "ALL"),
				logger.String("upstream", route.Upstream),
			)
		}
	}
}
