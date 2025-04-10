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
	Path            string                  `yaml:"path"`
	Methods         []string                `yaml:"methods"`
	Upstream        string                  `yaml:"upstream"`
	StripPrefix     bool                    `yaml:"strip_prefix"`
	RequireAuth     bool                    `yaml:"require_auth"`
	Timeout         int                     `yaml:"timeout"`
	WebSocket       *WebSocketConfig        `yaml:"websocket"`
	RateLimit       *RateLimitConfig        `yaml:"rate_limit"`
	Cache           *RouteCacheConfig       `yaml:"cache"`
	CircuitBreaker  *CircuitBreakerSettings `yaml:"circuit_breaker"`
	RetryPolicy     *RetryPolicy            `yaml:"retry_policy"`
	LoadBalancing   *LoadBalancingConfig    `yaml:"load_balancing"`
	HeaderTransform *HeaderTransform        `yaml:"header_transform"`
	URLRewrite      *URLRewrite             `yaml:"url_rewrite"`
	ErrorHandling   *ErrorHandling          `yaml:"error_handling"`
	Compression     bool                    `yaml:"compression"`
	IPWhitelist     []string                `yaml:"ip_whitelist"`
	IPBlacklist     []string                `yaml:"ip_blacklist"`
}

// RouteCacheConfig contains cache configuration for a route
type RouteCacheConfig struct {
	Enabled            bool `yaml:"enabled"`
	TTL                int  `yaml:"ttl"`
	CacheAuthenticated bool `yaml:"cache_authenticated"`
}

// RetryPolicy represents retry configuration for a route
type RetryPolicy struct {
	Enabled       bool     `yaml:"enabled"`
	Attempts      int      `yaml:"attempts"`
	PerTryTimeout int      `yaml:"per_try_timeout"`
	RetryOn       []string `yaml:"retry_on"`
}

// LoadBalancingConfig represents load balancing configuration for a route
type LoadBalancingConfig struct {
	Method      string   `yaml:"method"`
	HealthCheck bool     `yaml:"health_check"`
	Endpoints   []string `yaml:"endpoints"`
}

// HeaderTransform represents header transformation configuration
type HeaderTransform struct {
	Request  map[string]string `yaml:"request"`
	Response map[string]string `yaml:"response"`
	Remove   []string          `yaml:"remove"`
}

// URLRewrite represents URL rewriting configuration
type URLRewrite struct {
	Patterns []URLRewritePattern `yaml:"patterns"`
}

// URLRewritePattern represents a URL rewrite pattern
type URLRewritePattern struct {
	Match       string `yaml:"match"`
	Replacement string `yaml:"replacement"`
}

// ErrorHandling represents error handling configuration
type ErrorHandling struct {
	DefaultMessage string         `yaml:"default_message"`
	StatusCodes    map[int]string `yaml:"status_codes"`
	Templates      map[int]string `yaml:"templates"`
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

		// Set defaults for retry policy
		if route.RetryPolicy != nil && route.RetryPolicy.Enabled {
			if route.RetryPolicy.Attempts == 0 {
				routeConfig.Routes[i].RetryPolicy.Attempts = 3
			}
			if route.RetryPolicy.PerTryTimeout == 0 {
				routeConfig.Routes[i].RetryPolicy.PerTryTimeout = 5
			}
		}

		// Set defaults for circuit breaker
		if route.CircuitBreaker != nil && route.CircuitBreaker.Enabled {
			if route.CircuitBreaker.Threshold == 0 {
				routeConfig.Routes[i].CircuitBreaker.Threshold = 5
			}
			if route.CircuitBreaker.Timeout == 0 {
				routeConfig.Routes[i].CircuitBreaker.Timeout = 30
			}
		}

		// Set defaults for cache
		if route.Cache != nil && route.Cache.Enabled {
			if route.Cache.TTL == 0 {
				routeConfig.Routes[i].Cache.TTL = 60
			}
		}
	}

	return &routeConfig, nil
}
