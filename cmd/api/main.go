package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/server"
	"api-gateway/pkg/logger"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", getEnv("CONFIG_PATH", "configs/config.yaml"), "path to config file")
	routesPath := flag.String("routes", getEnv("ROUTES_PATH", "configs/routes.yaml"), "path to routes file")
	flag.Parse()

	// Initialize logger
	log := logger.NewLogger()
	log.Info("Starting API Gateway...")

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatal("Failed to load config", logger.Error(err))
	}

	// Load routes configuration
	routes, err := config.LoadRoutes(*routesPath)
	if err != nil {
		log.Fatal("Failed to load routes", logger.Error(err))
	}

	// Log configuration file paths
	log.Info("Configuration loaded",
		logger.String("config_path", *configPath),
		logger.String("routes_path", *routesPath),
	)

	// Create and start server
	srv := server.NewServer(cfg, routes, log)
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatal("Failed to start server", logger.Error(err))
		}
	}()

	log.Info("API Gateway is running", logger.String("address", cfg.Server.Address))

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down API Gateway...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		log.Fatal("Server forced to shutdown", logger.Error(err))
	}

	log.Info("API Gateway has been shutdown gracefully")
}

// getEnv retrieves environment variable or returns the provided default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	// Convert to absolute path if not already
	if !filepath.IsAbs(value) {
		if absPath, err := filepath.Abs(value); err == nil {
			return absPath
		}
	}

	return value
}
