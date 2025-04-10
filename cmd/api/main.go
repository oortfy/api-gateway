package main

import (
	"context"
	"fmt"
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
	// Load configuration
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger with configuration
	logConfig := logger.Config{
		Level:           os.Getenv("LOG_LEVEL"),
		Format:          os.Getenv("LOG_FORMAT"),
		Output:          "stdout",
		ProductionMode:  true,
		StacktraceLevel: "error",
		Sampling: &logger.SamplingConfig{
			Enabled:    true,
			Initial:    100,
			Thereafter: 100,
		},
		Fields: map[string]string{
			"service":     "api-gateway",
			"environment": os.Getenv("ENV"),
			"version":     os.Getenv("VERSION"),
		},
		Redact: []string{
			"jwt_secret",
			"api_key",
			"authorization",
			"password",
			"token",
		},
		MaxStacktraceLen: 2048,
	}

	log := logger.NewLogger(logConfig)

	// Load route configuration
	routes, err := config.LoadRoutes("configs/routes.yaml")
	if err != nil {
		log.Fatal("Failed to load route config",
			logger.Error(err),
			logger.String("config_file", "configs/routes.yaml"))
	}

	// Create and start server
	server := server.NewServer(cfg, routes, log)
	if err := server.Start(); err != nil {
		log.Fatal("Server failed",
			logger.Error(err))
	}

	log.Info("API Gateway is running", logger.String("address", cfg.Server.Address))

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down API Gateway...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
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
