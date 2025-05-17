package config

import "time"

// GRPCConfig holds gRPC-specific configuration
type GRPCConfig struct {
	// Enabled controls whether the gRPC server should be started
	Enabled bool `yaml:"enabled" default:"false"`

	// MaxIdleTime is the maximum time a connection can be idle before being closed
	MaxIdleTime time.Duration `yaml:"max_idle_time" default:"5m"`

	// MaxConnections is the maximum number of connections to keep in the pool
	MaxConnections int `yaml:"max_connections" default:"100"`

	// MaxRecvMsgSize is the maximum message size in bytes that can be received
	MaxRecvMsgSize int `yaml:"max_recv_msg_size" default:"16777216"` // 16MB

	// MaxSendMsgSize is the maximum message size in bytes that can be sent
	MaxSendMsgSize int `yaml:"max_send_msg_size" default:"16777216"` // 16MB

	// EnableReflection enables server reflection (useful for development)
	EnableReflection bool `yaml:"enable_reflection" default:"false"`

	// KeepAliveTime is the interval between keep-alive probes
	KeepAliveTime time.Duration `yaml:"keepalive_time" default:"30s"`

	// KeepAliveTimeout is how long to wait before closing an unresponsive connection
	KeepAliveTimeout time.Duration `yaml:"keepalive_timeout" default:"10s"`
}

// DefaultGRPCConfig returns the default gRPC configuration
func DefaultGRPCConfig() *GRPCConfig {
	return &GRPCConfig{
		Enabled:          false,
		MaxIdleTime:      5 * time.Minute,
		MaxConnections:   100,
		MaxRecvMsgSize:   16 * 1024 * 1024, // 16MB
		MaxSendMsgSize:   16 * 1024 * 1024, // 16MB
		EnableReflection: false,
		KeepAliveTime:    30 * time.Second,
		KeepAliveTimeout: 10 * time.Second,
	}
}
