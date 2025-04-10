package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"api-gateway/internal/auth"
	"api-gateway/internal/config"
	"api-gateway/internal/handlers"
	"api-gateway/internal/middleware"
	"api-gateway/internal/proxy"
	"api-gateway/pkg/logger"

	"github.com/gorilla/mux"
)

// Server represents the API Gateway server
type Server struct {
	config         *config.Config
	routes         *config.RouteConfig
	log            logger.Logger
	httpServer     *http.Server
	router         *mux.Router
	authService    *auth.AuthService
	httpProxy      *proxy.HTTPProxy
	wsProxy        *proxy.WSProxy
	authMiddleware *middleware.AuthMiddleware
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config, routes *config.RouteConfig, log logger.Logger) *Server {
	router := mux.NewRouter()

	// Initialize services
	authService := auth.NewAuthService(&cfg.Auth, log)
	httpProxy := proxy.NewHTTPProxy(cfg, routes, log)
	wsProxy := proxy.NewWSProxy(cfg, routes, log)
	authMiddleware := middleware.NewAuthMiddleware(authService, &cfg.Auth, log)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return &Server{
		config:         cfg,
		routes:         routes,
		log:            log,
		httpServer:     httpServer,
		router:         router,
		authService:    authService,
		httpProxy:      httpProxy,
		wsProxy:        wsProxy,
		authMiddleware: authMiddleware,
	}
}

// Start initializes and starts the server
func (s *Server) Start() error {
	// Register routes
	s.registerRoutes()

	// Start HTTP server
	s.log.Info("Starting server", logger.String("address", s.config.Server.Address))
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.log.Error("Failed to start server", logger.Error(err))
		return err
	}

	return nil
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
	routeRouter := s.router.PathPrefix(route.Path).Subrouter()

	// Register the appropriate handlers based on whether it's a WebSocket route or not
	if route.WebSocket != nil && route.WebSocket.Enabled {
		// WebSocket handler
		wsHandler := s.wsProxy.ProxyWebSocket(route)

		// Apply authentication middleware if required
		if route.RequireAuth {
			wsHandler = s.authMiddleware.Authenticate(wsHandler, route)
		}

		// Register the handler for the WebSocket-specific path or the general route path
		wsPath := route.WebSocket.Path
		if wsPath == "" {
			// If no specific path is provided, use the general path
			routeRouter.PathPrefix("/").Handler(wsHandler)
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
	} else {
		// HTTP handler
		httpHandler := s.httpProxy.ProxyRequest(route)

		// Apply authentication middleware if required
		if route.RequireAuth {
			httpHandler = s.authMiddleware.Authenticate(httpHandler, route)
		}

		// If methods are specified, register the handler for each method
		if len(route.Methods) > 0 {
			for _, method := range route.Methods {
				routeRouter.PathPrefix("/").Handler(httpHandler).Methods(method)
				s.log.Info("Registered route",
					logger.String("path", fmt.Sprintf("%s/*", route.Path)),
					logger.String("method", method),
					logger.String("upstream", route.Upstream),
				)
			}
		} else {
			// Otherwise, register for all methods
			routeRouter.PathPrefix("/").Handler(httpHandler)
			s.log.Info("Registered route",
				logger.String("path", fmt.Sprintf("%s/*", route.Path)),
				logger.String("method", "ALL"),
				logger.String("upstream", route.Upstream),
			)
		}
	}
}
