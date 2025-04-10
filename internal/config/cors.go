package config

// CORSConfig contains CORS configuration
type CORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowAllOrigins  bool     `yaml:"allow_all_origins"`
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}
