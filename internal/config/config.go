package config

import (
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config contains all configuration for the application
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Auth     AuthConfig     `yaml:"auth"`
	Logging  LoggingConfig  `yaml:"logging"`
	Security SecurityConfig `yaml:"security"`
	Cache    CacheConfig    `yaml:"cache"`
	Cors     CorsConfig     `yaml:"cors"`
	Metrics  MetricsConfig  `yaml:"metrics"`
	Tracing  TracingConfig  `yaml:"tracing"`
	Etcd     EtcdConfig     `yaml:"etcd"`
}

// ServerConfig contains server configuration
type ServerConfig struct {
	Address           string `yaml:"address"`
	ReadTimeout       int    `yaml:"read_timeout"`
	WriteTimeout      int    `yaml:"write_timeout"`
	IdleTimeout       int    `yaml:"idle_timeout"`
	MaxHeaderBytes    int    `yaml:"max_header_bytes"`
	EnableHTTP2       bool   `yaml:"enable_http2"`
	EnableCompression bool   `yaml:"enable_compression"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	JWTSecret           string `yaml:"jwt_secret"`
	JWTExpiryHours      int    `yaml:"jwt_expiry_hours"`
	APIKeyValidationURL string `yaml:"api_key_validation_url"`
	APIKeyHeader        string `yaml:"api_key_header"`
	JWTHeader           string `yaml:"jwt_header"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level        string `yaml:"level"`
	Format       string `yaml:"format"`
	Output       string `yaml:"output"`
	EnableAccess bool   `yaml:"enable_access_log"`
}

// SecurityConfig contains security configuration
type SecurityConfig struct {
	TLS                      TLSConfig `yaml:"tls"`
	EnableXSSProtection      bool      `yaml:"enable_xss_protection"`
	EnableFrameDeny          bool      `yaml:"enable_frame_deny"`
	EnableContentTypeNosniff bool      `yaml:"enable_content_type_nosniff"`
	EnableHSTS               bool      `yaml:"enable_hsts"`
	HSTSMaxAge               int       `yaml:"hsts_max_age"`
	TrustedProxies           []string  `yaml:"trusted_proxies"`
	IPWhitelist              []string  `yaml:"ip_whitelist"`
	IPBlacklist              []string  `yaml:"ip_blacklist"`
	MaxBodySize              int64     `yaml:"max_body_size"`
}

// TLSConfig contains TLS configuration
type TLSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	CertFile         string   `yaml:"cert_file"`
	KeyFile          string   `yaml:"key_file"`
	MinVersion       string   `yaml:"min_version"`
	MaxVersion       string   `yaml:"max_version"`
	CipherSuites     []string `yaml:"cipher_suites"`
	CurvePreferences []string `yaml:"curve_preferences"`
}

// CacheConfig contains caching configuration
type CacheConfig struct {
	Enabled       bool     `yaml:"enabled"`
	DefaultTTL    int      `yaml:"default_ttl"`
	MaxTTL        int      `yaml:"max_ttl"`
	MaxSize       int      `yaml:"max_size"`
	IncludeHost   bool     `yaml:"include_host"`
	VaryHeaders   []string `yaml:"vary_headers"`
	PurgeEndpoint string   `yaml:"purge_endpoint"`
}

// CorsConfig contains CORS configuration
type CorsConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowAllOrigins  bool     `yaml:"allow_all_origins"`
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

// MetricsConfig contains metrics configuration
type MetricsConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Endpoint      string `yaml:"endpoint"`
	IncludeSystem bool   `yaml:"include_system"`
}

// TracingConfig contains tracing configuration
type TracingConfig struct {
	Enabled     bool    `yaml:"enabled"`
	Provider    string  `yaml:"provider"`
	Endpoint    string  `yaml:"endpoint"`
	ServiceName string  `yaml:"service_name"`
	SampleRate  float64 `yaml:"sample_rate"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Requests int    `yaml:"requests"`
	Period   string `yaml:"period"`
}

// CacheSettings represents cache settings for a route
type CacheSettings struct {
	Enabled            bool `yaml:"enabled"`
	TTL                int  `yaml:"ttl"`
	CacheAuthenticated bool `yaml:"cache_authenticated"`
}

// CircuitBreakerSettings represents circuit breaker settings for a route
type CircuitBreakerSettings struct {
	Enabled       bool `yaml:"enabled"`
	Threshold     int  `yaml:"threshold"`
	Timeout       int  `yaml:"timeout"`
	MaxConcurrent int  `yaml:"max_concurrent"`
}

// WebSocketConfig represents websocket-specific configuration
type WebSocketConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Path         string `yaml:"path"`
	UpstreamPath string `yaml:"upstream_path"`
}

type EtcdConfig struct {
	Hosts string `yaml:"hosts"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	configFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer configFile.Close()

	data, err := io.ReadAll(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Replace environment variables in the format ${VAR_NAME}
	data = replaceEnvVars(data)

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	setConfigDefaults(&config)

	return &config, nil
}

// setConfigDefaults sets default values for the configuration
func setConfigDefaults(config *Config) {
	// Server defaults
	if config.Server.ReadTimeout == 0 {
		config.Server.ReadTimeout = 30 // Default read timeout of 30 seconds
	}
	if config.Server.WriteTimeout == 0 {
		config.Server.WriteTimeout = 30 // Default write timeout of 30 seconds
	}
	if config.Server.IdleTimeout == 0 {
		config.Server.IdleTimeout = 120 // Default idle timeout of 120 seconds
	}
	if config.Server.MaxHeaderBytes == 0 {
		config.Server.MaxHeaderBytes = 1 << 20 // Default max header bytes (1MB)
	}

	// Auth defaults
	if config.Auth.JWTHeader == "" {
		config.Auth.JWTHeader = "Authorization"
	}
	if config.Auth.APIKeyHeader == "" {
		config.Auth.APIKeyHeader = "X-API-Auth-Token"
	}

	// Cache defaults
	if config.Cache.DefaultTTL == 0 {
		config.Cache.DefaultTTL = 60 // Default TTL of 60 seconds
	}
	if config.Cache.MaxTTL == 0 {
		config.Cache.MaxTTL = 3600 // Default max TTL of 1 hour
	}
	if config.Cache.MaxSize == 0 {
		config.Cache.MaxSize = 1000 // Default max size of 1000 entries
	}
	if len(config.Cache.VaryHeaders) == 0 {
		config.Cache.VaryHeaders = []string{"Accept", "Accept-Encoding"}
	}

	// CORS defaults
	if len(config.Cors.AllowedMethods) == 0 {
		config.Cors.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	}
	if len(config.Cors.AllowedHeaders) == 0 {
		config.Cors.AllowedHeaders = []string{
			"Origin", "Content-Type", "Accept", "Authorization",
			"X-Requested-With", "X-API-Auth-Token",
		}
	}
	if config.Cors.MaxAge == 0 {
		config.Cors.MaxAge = 86400 // Default max age of 24 hours
	}

	// Security defaults
	if config.Security.HSTSMaxAge == 0 {
		config.Security.HSTSMaxAge = 31536000 // Default HSTS max age of 1 year
	}
	if config.Security.MaxBodySize == 0 {
		config.Security.MaxBodySize = 10 << 20 // Default max body size of 10MB
	}

	// Metrics defaults
	if config.Metrics.Endpoint == "" {
		config.Metrics.Endpoint = "/metrics"
	}

	// Tracing defaults
	if config.Tracing.Provider == "" {
		config.Tracing.Provider = "jaeger"
	}
	if config.Tracing.ServiceName == "" {
		config.Tracing.ServiceName = "api-gateway"
	}
	if config.Tracing.SampleRate == 0 {
		config.Tracing.SampleRate = 0.1 // Default sample rate of 10%
	}
}

// replaceEnvVars replaces environment variables in the format ${VAR_NAME} with their values
func replaceEnvVars(data []byte) []byte {
	content := string(data)
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) != 2 {
			continue
		}
		varName, varValue := pair[0], pair[1]
		placeholder := fmt.Sprintf("${%s}", varName)
		content = strings.ReplaceAll(content, placeholder, varValue)
	}
	return []byte(content)
}
