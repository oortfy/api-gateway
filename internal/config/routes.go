package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RouteConfig represents a route configuration in routes.yaml
type RouteConfig struct {
	Routes []Route `yaml:"routes"`
}

// Route represents a single API route
type Route struct {
	Path         string           `yaml:"path"`
	Methods      []string         `yaml:"methods"`
	Upstream     string           `yaml:"upstream"`
	StripPrefix  bool             `yaml:"strip_prefix"`
	RequireAuth  bool             `yaml:"require_auth"`
	AllowedRoles []string         `yaml:"allowed_roles"`
	WebSocket    *WebSocketConfig `yaml:"websocket"`
	Timeout      int              `yaml:"timeout"`
	RateLimit    *RateLimitConfig `yaml:"rate_limit"`
}

// WebSocketConfig represents websocket-specific configuration
type WebSocketConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Path         string `yaml:"path"`
	UpstreamPath string `yaml:"upstream_path"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Requests int    `yaml:"requests"`
	Period   string `yaml:"period"`
}

// LoadRoutes loads route configurations from a YAML file
func LoadRoutes(path string) (*RouteConfig, error) {
	routesFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open routes file: %w", err)
	}
	defer routesFile.Close()

	var routeConfig RouteConfig
	decoder := yaml.NewDecoder(routesFile)
	if err := decoder.Decode(&routeConfig); err != nil {
		return nil, fmt.Errorf("failed to parse routes file: %w", err)
	}

	// Validate routes
	for i, route := range routeConfig.Routes {
		if route.Path == "" {
			return nil, fmt.Errorf("route at index %d is missing 'path'", i)
		}
		if route.Upstream == "" {
			return nil, fmt.Errorf("route at index %d is missing 'upstream'", i)
		}
		if len(route.Methods) == 0 {
			// Default to all methods if none specified
			routeConfig.Routes[i].Methods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
		}
		if route.Timeout == 0 {
			// Default timeout of 30 seconds
			routeConfig.Routes[i].Timeout = 30
		}
	}

	return &routeConfig, nil
}
