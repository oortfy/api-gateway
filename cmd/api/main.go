//go:build !buildvcs
// +build !buildvcs

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/server"
	"api-gateway/pkg/logger"
)

// getEnvOrDefault gets environment variable or returns the default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger with configuration, using environment variables with fallback to config values
	logConfig := logger.Config{
		Level:           getEnvOrDefault("LOG_LEVEL", cfg.Logging.Level),
		Format:          getEnvOrDefault("LOG_FORMAT", cfg.Logging.Format),
		Output:          getEnvOrDefault("LOG_OUTPUT", cfg.Logging.Output),
		ProductionMode:  true,
		StacktraceLevel: "error",
		Sampling: &logger.SamplingConfig{
			Enabled:    true,
			Initial:    100,
			Thereafter: 100,
		},
		Fields: map[string]string{
			"service":     getEnvOrDefault("SERVICE", "api-gateway"),
			"environment": getEnvOrDefault("ENV", "production"),
			"version":     getEnvOrDefault("VERSION", "1.0.0"),
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
	routes, err := config.LoadRoutes("routes.yaml")
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

	log.Info("API Gateway is running",
		logger.String("address", cfg.Server.Address),
		logger.String("env", logConfig.Fields["environment"]),
		logger.String("version", logConfig.Fields["version"]))

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
