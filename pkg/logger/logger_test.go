package logger

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureOutput(f func()) string {
	// Redirect stdout to capture output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run the function that produces output
	f()

	// Reset stdout and read captured output
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)

	return buf.String()
}

func TestNewLogger(t *testing.T) {
	// Test with default config
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}

	logger := NewLogger(cfg)
	assert.NotNil(t, logger)
	assert.IsType(t, &zapLogger{}, logger)
}

func TestLoggerWithFields(t *testing.T) {
	cfg := Config{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	}

	logger := NewLogger(cfg)
	childLogger := logger.With(String("service", "test-service"), Int("port", 8080))

	assert.NotNil(t, childLogger)
	assert.IsType(t, &zapLogger{}, childLogger)
}

func TestLogLevels(t *testing.T) {
	// Create a temporary log file
	tmpfile, err := os.CreateTemp("", "log")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	cfg := Config{
		Level:  "debug",
		Format: "json",
		Output: tmpfile.Name(),
	}

	logger := NewLogger(cfg)

	// Test writing log messages
	logger.Debug("debug message", String("key1", "value1"))
	logger.Info("info message", Int("key2", 123))
	logger.Warn("warning message", Bool("key3", true))
	logger.Error("error message", Error(assert.AnError))

	// Close the file and read its contents
	tmpfile.Close()
	content, err := os.ReadFile(tmpfile.Name())
	require.NoError(t, err)

	// Split the content into lines
	lines := bytes.Split(content, []byte("\n"))

	// Check each log line is properly formatted
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var logEntry map[string]interface{}
		err := json.Unmarshal(line, &logEntry)
		assert.NoError(t, err, "Log line should be valid JSON")

		assert.Contains(t, logEntry, "level", "Log entry should contain level field")
		assert.Contains(t, logEntry, "timestamp", "Log entry should contain timestamp field")
		assert.Contains(t, logEntry, "msg", "Log entry should contain msg field")
	}
}

func TestConvenienceFunctions(t *testing.T) {
	// Test String field
	field := String("key", "value")
	assert.Equal(t, "key", field.Key)
	assert.Equal(t, "value", field.Value)

	// Test Int field
	field = Int("key", 123)
	assert.Equal(t, "key", field.Key)
	assert.Equal(t, 123, field.Value)

	// Test Bool field
	field = Bool("key", true)
	assert.Equal(t, "key", field.Key)
	assert.Equal(t, true, field.Value)

	// Test Error field
	testErr := assert.AnError
	field = Error(testErr)
	assert.Equal(t, "error", field.Key)
	assert.Equal(t, testErr.Error(), field.Value)

	// Test Any field
	obj := map[string]string{"foo": "bar"}
	field = Any("key", obj)
	assert.Equal(t, "key", field.Key)
	assert.Equal(t, obj, field.Value)
}
