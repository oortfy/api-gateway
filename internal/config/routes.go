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
	Path          string               `yaml:"path"`
	Methods       []string             `yaml:"methods"`
	Upstream      string               `yaml:"upstream"`
	Protocol      string               `yaml:"protocol"`
	StripPrefix   bool                 `yaml:"strip_prefix"`
	Timeout       int                  `yaml:"timeout"`
	WebSocket     *WebSocketConfig     `yaml:"websocket"`
	LoadBalancing *LoadBalancingConfig `yaml:"load_balancing"`
	ErrorHandling *ErrorHandling       `yaml:"error_handling"`
	Compression   bool                 `yaml:"compression"`
	IPWhitelist   []string             `yaml:"ip_whitelist"`
	IPBlacklist   []string             `yaml:"ip_blacklist"`
	Middlewares   *Middlewares         `yaml:"middlewares"`
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
	Method            string             `yaml:"method"`
	HealthCheck       bool               `yaml:"health_check"`
	Endpoints         []string           `yaml:"endpoints"`
	Driver            string             `yaml:"driver"`
	Discoveries       *Discoveries       `yaml:"discoveries"`
	HealthCheckConfig *HealthCheckConfig `yaml:"health_check_config"`
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Path               string `yaml:"path"`
	Interval           int    `yaml:"interval"`
	Timeout            int    `yaml:"timeout"`
	HealthyThreshold   int    `yaml:"healthy_threshold"`
	UnhealthyThreshold int    `yaml:"unhealthy_threshold"`
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

type Middlewares struct {
	RequireAuth     bool                    `yaml:"require_auth"`
	RateLimit       *RateLimitConfig        `yaml:"rate_limit"`
	Cache           *RouteCacheConfig       `yaml:"cache"`
	CircuitBreaker  *CircuitBreakerSettings `yaml:"circuit_breaker"`
	RetryPolicy     *RetryPolicy            `yaml:"retry_policy"`
	HeaderTransform *HeaderTransform        `yaml:"header_transform"`
	URLRewrite      *URLRewrite             `yaml:"url_rewrite"`
}

type Discoveries struct {
	Name      string `yaml:"name"`
	Prefix    string `yaml:"prefix"`
	FailLimit int    `yaml:"fail_limit"`
}

// LoadRoutes loads route configurations from a YAML file
func LoadRoutes(path string) (*RouteConfig, error) {
	var routeConfig RouteConfig
	var err error
	var data []byte

	// Development mode: Try reading from filesystem first
	if os.Getenv("GO_ENV") == "development" {
		file, err := os.Open("configs/" + path)
		if err == nil {
			defer file.Close()
			decoder := yaml.NewDecoder(file)
			if err = decoder.Decode(&routeConfig); err == nil {
				goto VALIDATE_ROUTES
			}
		}
		// Continue to embedded config if file read fails
	}

	// Production mode: Read from embedded packr box
	data, err = configBox.Find(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load routes config (both file and embedded): %w", err)
	}

	if err := yaml.Unmarshal(data, &routeConfig); err != nil {
		return nil, fmt.Errorf("failed to parse embedded routes config: %w", err)
	}

VALIDATE_ROUTES:
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
		if route.Middlewares.RetryPolicy != nil && route.Middlewares.RetryPolicy.Enabled {
			if route.Middlewares.RetryPolicy.Attempts == 0 {
				routeConfig.Routes[i].Middlewares.RetryPolicy.Attempts = 3
			}
			if route.Middlewares.RetryPolicy.PerTryTimeout == 0 {
				routeConfig.Routes[i].Middlewares.RetryPolicy.PerTryTimeout = 5
			}
		}

		// Set defaults for circuit breaker
		if route.Middlewares.CircuitBreaker != nil && route.Middlewares.CircuitBreaker.Enabled {
			if route.Middlewares.CircuitBreaker.Threshold == 0 {
				routeConfig.Routes[i].Middlewares.CircuitBreaker.Threshold = 5
			}
			if route.Middlewares.CircuitBreaker.Timeout == 0 {
				routeConfig.Routes[i].Middlewares.CircuitBreaker.Timeout = 30
			}
		}

		// Set defaults for cache
		if route.Middlewares.Cache != nil && route.Middlewares.Cache.Enabled {
			if route.Middlewares.Cache.TTL == 0 {
				routeConfig.Routes[i].Middlewares.Cache.TTL = 60
			}
		}
	}

	return &routeConfig, nil
}
