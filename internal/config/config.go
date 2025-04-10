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
	Server  ServerConfig  `yaml:"server"`
	Auth    AuthConfig    `yaml:"auth"`
	Logging LoggingConfig `yaml:"logging"`
}

// ServerConfig contains server configuration
type ServerConfig struct {
	Address      string `yaml:"address"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
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
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
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

	// Set defaults if not provided
	if config.Server.ReadTimeout == 0 {
		config.Server.ReadTimeout = 30 // Default read timeout of 30 seconds
	}
	if config.Server.WriteTimeout == 0 {
		config.Server.WriteTimeout = 30 // Default write timeout of 30 seconds
	}
	if config.Auth.JWTHeader == "" {
		config.Auth.JWTHeader = "Authorization"
	}
	if config.Auth.APIKeyHeader == "" {
		config.Auth.APIKeyHeader = "X-API-Auth-Token"
	}

	return &config, nil
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
