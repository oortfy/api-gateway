package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger interface defines the logging methods
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)
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

// NewLogger creates a new logger instance
func NewLogger() Logger {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.StacktraceKey = "stacktrace"

	logger, err := config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}

	return &zapLogger{
		logger: logger,
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
