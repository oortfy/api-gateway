package logger

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config represents logger configuration
type Config struct {
	Level            string
	Format           string
	Output           string
	EnableAccessLog  bool
	ProductionMode   bool
	StacktraceLevel  string
	Sampling         *SamplingConfig
	Fields           map[string]string
	Redact           []string
	MaxStacktraceLen int
}

// SamplingConfig represents sampling configuration
type SamplingConfig struct {
	Enabled    bool
	Initial    int
	Thereafter int
}

// Logger interface defines the logging methods
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)
	With(fields ...Field) Logger
}

// Field represents a log field
type Field struct {
	Key   string
	Value interface{}
}

// zapLogger implements the Logger interface using zap
type zapLogger struct {
	logger *zap.Logger
}

// NewLogger creates a new logger instance with configuration
func NewLogger(cfg Config) Logger {
	config := zap.NewProductionConfig()

	// Configure log level
	level, err := zapcore.ParseLevel(cfg.Level)
	if err == nil {
		config.Level = zap.NewAtomicLevelAt(level)
	}

	// Configure encoding
	if cfg.Format == "console" {
		config.Encoding = "console"
	} else {
		config.Encoding = "json"
	}

	// Configure output paths
	if cfg.Output != "" {
		config.OutputPaths = []string{cfg.Output}
	}

	// Configure encoder
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.StacktraceKey = "stacktrace"

	// Configure sampling if enabled
	if cfg.Sampling != nil && cfg.Sampling.Enabled {
		config.Sampling = &zap.SamplingConfig{
			Initial:    cfg.Sampling.Initial,
			Thereafter: cfg.Sampling.Thereafter,
		}
	}

	// Configure stacktrace level
	stackLevel, err := zapcore.ParseLevel(cfg.StacktraceLevel)
	if err == nil {
		config.Development = false
		config.DisableStacktrace = stackLevel == zapcore.FatalLevel
	}

	// Create options
	opts := []zap.Option{zap.AddCallerSkip(1)}

	// Add default fields
	if len(cfg.Fields) > 0 {
		fields := make([]zap.Field, 0, len(cfg.Fields))
		for k, v := range cfg.Fields {
			fields = append(fields, zap.String(k, v))
		}
		opts = append(opts, zap.Fields(fields...))
	}

	// Configure field redaction
	if len(cfg.Redact) > 0 {
		opts = append(opts, zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewSamplerWithOptions(core, time.Second, cfg.Sampling.Initial, cfg.Sampling.Thereafter)
		}))
	}

	logger, err := config.Build(opts...)
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}

	return &zapLogger{
		logger: logger,
	}
}

// With creates a child logger with the given fields
func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{
		logger: l.logger.With(l.convertFields(fields...)...),
	}
}

// Debug logs a debug message
func (l *zapLogger) Debug(msg string, fields ...Field) {
	l.logger.Debug(msg, l.convertFields(fields...)...)
}

// Info logs an info message
func (l *zapLogger) Info(msg string, fields ...Field) {
	l.logger.Info(msg, l.convertFields(fields...)...)
}

// Warn logs a warning message
func (l *zapLogger) Warn(msg string, fields ...Field) {
	l.logger.Warn(msg, l.convertFields(fields...)...)
}

// Error logs an error message
func (l *zapLogger) Error(msg string, fields ...Field) {
	l.logger.Error(msg, l.convertFields(fields...)...)
}

// Fatal logs a fatal message and terminates the program
func (l *zapLogger) Fatal(msg string, fields ...Field) {
	l.logger.Fatal(msg, l.convertFields(fields...)...)
}

// convertFields converts logger.Field to zap.Field
func (l *zapLogger) convertFields(fields ...Field) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields))
	for _, field := range fields {
		zapFields = append(zapFields, zap.Any(field.Key, field.Value))
	}
	return zapFields
}

// Convenience functions to create fields
func String(key string, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func Error(err error) Field {
	return Field{Key: "error", Value: err.Error()}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}
