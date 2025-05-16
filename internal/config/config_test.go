package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configData := `
server:
  address: ":8080"
  read_timeout: 10
  write_timeout: 10
  idle_timeout: 30
  max_header_bytes: 1048576
  enable_http2: true
  enable_compression: true
auth:
  jwt_secret: "test-secret"
  jwt_expiry_hours: 24
  api_key_validation_url: "http://auth-service/validate"
  api_key_header: "X-API-Key"
  jwt_header: "Authorization"
logging:
  level: "debug"
  format: "json"
  output: "stdout"
  enable_access_log: true
routes:
  - path: "/api/test"
    upstream: "http://test-service:8080"
    methods: ["GET", "POST"]
    strip_prefix: true
`

	err := os.WriteFile(configPath, []byte(configData), 0644)
	require.NoError(t, err)

	// Test loading the config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.NotNil(t, config)

	// Test server config section
	assert.Equal(t, ":8080", config.Server.Address)
	assert.Equal(t, 10, config.Server.ReadTimeout)
	assert.Equal(t, 10, config.Server.WriteTimeout)
	assert.Equal(t, 30, config.Server.IdleTimeout)
	assert.Equal(t, 1048576, config.Server.MaxHeaderBytes)
	assert.True(t, config.Server.EnableHTTP2)
	assert.True(t, config.Server.EnableCompression)

	// Test auth config section
	assert.Equal(t, "test-secret", config.Auth.JWTSecret)
	assert.Equal(t, 24, config.Auth.JWTExpiryHours)
	assert.Equal(t, "http://auth-service/validate", config.Auth.APIKeyValidationURL)
	assert.Equal(t, "X-API-Key", config.Auth.APIKeyHeader)
	assert.Equal(t, "Authorization", config.Auth.JWTHeader)

	// Test logging config section
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, "stdout", config.Logging.Output)
	assert.True(t, config.Logging.EnableAccess)

	// Test routes section
	require.Len(t, config.Routes, 1)
	assert.Equal(t, "/api/test", config.Routes[0].Path)
	assert.Equal(t, "http://test-service:8080", config.Routes[0].Upstream)
	assert.ElementsMatch(t, []string{"GET", "POST"}, config.Routes[0].Methods)
	assert.True(t, config.Routes[0].StripPrefix)
}

func TestLoadConfigNonExistentFile(t *testing.T) {
	// Attempt to load a non-existent config file
	_, err := LoadConfig("nonexistent.yaml")
	assert.Error(t, err)
}

func TestLoadConfigWithEnvVars(t *testing.T) {
	// Set environment variables to test substitution
	os.Setenv("TEST_PORT", "9090")
	os.Setenv("TEST_SECRET", "env-secret")
	defer os.Unsetenv("TEST_PORT")
	defer os.Unsetenv("TEST_SECRET")

	// Create a temporary config file with environment variable references
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configData := `
server:
  address: ":${TEST_PORT}"
auth:
  jwt_secret: "${TEST_SECRET}"
`

	err := os.WriteFile(configPath, []byte(configData), 0644)
	require.NoError(t, err)

	// Test loading the config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify environment variables were substituted
	assert.Equal(t, ":9090", config.Server.Address)
	assert.Equal(t, "env-secret", config.Auth.JWTSecret)
}

func TestLoadConfigFromConfigsDir(t *testing.T) {
	// Save current working directory
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	// Create a temporary directory to simulate project structure
	tempDir := t.TempDir()
	configsDir := filepath.Join(tempDir, "configs")
	err = os.Mkdir(configsDir, 0755)
	require.NoError(t, err)

	// Create config file in configs directory
	configPath := filepath.Join(configsDir, "config.yaml")
	configData := `
server:
  address: ":8080"
`
	err = os.WriteFile(configPath, []byte(configData), 0644)
	require.NoError(t, err)

	// Change to temp directory to simulate running from project root
	err = os.Chdir(tempDir)
	require.NoError(t, err)
	defer os.Chdir(currentDir) // Restore original directory

	// Test loading the config by name only (should find it in configs/)
	config, err := LoadConfig("config.yaml")
	require.NoError(t, err)
	require.NotNil(t, config)

	assert.Equal(t, ":8080", config.Server.Address)
}

func TestParseConfig(t *testing.T) {
	// Test valid config
	validConfig := `
server:
  address: ":8080"
auth:
  jwt_secret: test-secret
logging:
  level: debug
`
	cfg, err := parseConfig([]byte(validConfig))
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.Server.Address)
	assert.Equal(t, "test-secret", cfg.Auth.JWTSecret)
	assert.Equal(t, "debug", cfg.Logging.Level)

	// Test invalid YAML
	invalidConfig := `
server:
  address: ":8080"
auth:
  jwt_secret: "unclosed string
`
	_, err = parseConfig([]byte(invalidConfig))
	assert.Error(t, err)
}

func TestSetConfigDefaults(t *testing.T) {
	// Test with empty config to check all defaults
	emptyConfig := &Config{}
	setConfigDefaults(emptyConfig)

	// Check server defaults
	assert.Equal(t, 30, emptyConfig.Server.ReadTimeout)
	assert.Equal(t, 30, emptyConfig.Server.WriteTimeout)
	assert.Equal(t, 120, emptyConfig.Server.IdleTimeout)
	assert.Equal(t, 1<<20, emptyConfig.Server.MaxHeaderBytes)

	// Check auth defaults
	assert.Equal(t, "Authorization", emptyConfig.Auth.JWTHeader)
	assert.Equal(t, "X-API-Auth-Token", emptyConfig.Auth.APIKeyHeader)

	// Check cache defaults
	assert.Equal(t, 60, emptyConfig.Cache.DefaultTTL)
	assert.Equal(t, 3600, emptyConfig.Cache.MaxTTL)
	assert.Equal(t, 1000, emptyConfig.Cache.MaxSize)
	assert.Equal(t, []string{"Accept", "Accept-Encoding"}, emptyConfig.Cache.VaryHeaders)

	// Check CORS defaults
	assert.Equal(t, []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, emptyConfig.Cors.AllowedMethods)
	assert.Contains(t, emptyConfig.Cors.AllowedHeaders, "Authorization")
	assert.Equal(t, 86400, emptyConfig.Cors.MaxAge)

	// Check security defaults
	assert.Equal(t, 31536000, emptyConfig.Security.HSTSMaxAge)
	assert.Equal(t, int64(10<<20), emptyConfig.Security.MaxBodySize)

	// Check metrics defaults
	assert.Equal(t, "/metrics", emptyConfig.Metrics.Endpoint)

	// Check tracing defaults
	assert.Equal(t, "jaeger", emptyConfig.Tracing.Provider)
	assert.Equal(t, "api-gateway", emptyConfig.Tracing.ServiceName)
	assert.Equal(t, 0.1, emptyConfig.Tracing.SampleRate)

	// Test with some values already set
	configWithValues := &Config{
		Server: ServerConfig{
			ReadTimeout: 60,
		},
		Auth: AuthConfig{
			JWTHeader: "X-JWT-Token",
		},
		Cache: CacheConfig{
			DefaultTTL: 120,
		},
	}
	setConfigDefaults(configWithValues)

	// Check that existing values are preserved
	assert.Equal(t, 60, configWithValues.Server.ReadTimeout)
	assert.Equal(t, "X-JWT-Token", configWithValues.Auth.JWTHeader)
	assert.Equal(t, 120, configWithValues.Cache.DefaultTTL)

	// Check that unset values got defaults
	assert.Equal(t, 30, configWithValues.Server.WriteTimeout)
	assert.Equal(t, "X-API-Auth-Token", configWithValues.Auth.APIKeyHeader)
}

func TestReplaceEnvVars(t *testing.T) {
	// Set some environment variables for testing
	os.Setenv("TEST_HOST", "localhost")
	os.Setenv("TEST_PORT", "8080")
	defer os.Unsetenv("TEST_HOST")
	defer os.Unsetenv("TEST_PORT")

	// Test config with env vars
	configWithEnvVars := `
server:
  address: "${TEST_HOST}:${TEST_PORT}"
auth:
  jwt_secret: "${TEST_SECRET:-default-secret}"
`

	replacedConfig := replaceEnvVars([]byte(configWithEnvVars))

	// Check that environment variables were replaced
	assert.Contains(t, string(replacedConfig), "localhost:8080")

	// TEST_SECRET is not set, so it should remain as is (we don't have full templating)
	assert.Contains(t, string(replacedConfig), "${TEST_SECRET:-default-secret}")
}
