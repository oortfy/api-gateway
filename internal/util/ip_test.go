package util

import (
	"api-gateway/pkg/logger"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockLogger implements the logger.Logger interface for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, args ...logger.Field) {}
func (m *mockLogger) Info(msg string, args ...logger.Field)  {}
func (m *mockLogger) Warn(msg string, args ...logger.Field)  {}
func (m *mockLogger) Error(msg string, args ...logger.Field) {}
func (m *mockLogger) Fatal(msg string, args ...logger.Field) {}
func (m *mockLogger) With(args ...logger.Field) logger.Logger {
	return m
}

func TestGetClientIP(t *testing.T) {
	testCases := []struct {
		name           string
		remoteAddr     string
		headers        map[string]string
		expectedResult string
	}{
		{
			name:           "x_real_ip",
			remoteAddr:     "10.0.0.1:1234",
			headers:        map[string]string{"X-Real-IP": "11.22.33.44"},
			expectedResult: "11.22.33.44",
		},
		{
			name:           "x_forwarded_for_single",
			remoteAddr:     "10.0.0.1:1234",
			headers:        map[string]string{"X-Forwarded-For": "22.33.44.55"},
			expectedResult: "22.33.44.55",
		},
		{
			name:           "x_forwarded_for_multiple",
			remoteAddr:     "10.0.0.1:1234",
			headers:        map[string]string{"X-Forwarded-For": "22.33.44.55, 66.77.88.99, 10.0.0.2"},
			expectedResult: "22.33.44.55",
		},
		{
			name:           "cloudflare",
			remoteAddr:     "10.0.0.1:1234",
			headers:        map[string]string{"CF-Connecting-IP": "99.88.77.66"},
			expectedResult: "99.88.77.66",
		},
		{
			name:           "true_client_ip",
			remoteAddr:     "10.0.0.1:1234",
			headers:        map[string]string{"True-Client-IP": "55.66.77.88"},
			expectedResult: "55.66.77.88",
		},
		{
			name:           "forwarded_ipv4",
			remoteAddr:     "10.0.0.1:1234",
			headers:        map[string]string{"Forwarded": "for=192.168.0.1;proto=https"},
			expectedResult: "192.168.0.1",
		},
		{
			name:           "forwarded_ipv4_with_port",
			remoteAddr:     "10.0.0.1:1234",
			headers:        map[string]string{"Forwarded": "for=192.168.0.1:8080;proto=https"},
			expectedResult: "192.168.0.1",
		},
		{
			name:           "forwarded_ipv6",
			remoteAddr:     "10.0.0.1:1234",
			headers:        map[string]string{"Forwarded": "for=[2001:db8:cafe::17];proto=https"},
			expectedResult: "2001:db8:cafe::17",
		},
		{
			name:           "remote_addr_only",
			remoteAddr:     "99.99.99.99:9999",
			headers:        map[string]string{},
			expectedResult: "99.99.99.99",
		},
		{
			name:           "remote_addr_without_port",
			remoteAddr:     "88.88.88.88",
			headers:        map[string]string{},
			expectedResult: "88.88.88.88",
		},
		{
			name:           "x_real_ip_with_unknown",
			remoteAddr:     "10.0.0.1:1234",
			headers:        map[string]string{"X-Real-IP": "unknown"},
			expectedResult: "10.0.0.1",
		},
		{
			name:       "multiple_headers_priority",
			remoteAddr: "10.0.0.1:1234",
			headers: map[string]string{
				"X-Real-IP":        "11.11.11.11",
				"X-Forwarded-For":  "22.22.22.22",
				"CF-Connecting-IP": "33.33.33.33",
				"True-Client-IP":   "44.44.44.44",
				"Forwarded":        "for=55.55.55.55",
			},
			expectedResult: "11.11.11.11", // X-Real-IP has highest priority
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			req.RemoteAddr = tc.remoteAddr

			// Set headers
			for name, value := range tc.headers {
				req.Header.Set(name, value)
			}

			// Get client IP
			clientIP := GetClientIP(req)
			assert.Equal(t, tc.expectedResult, clientIP)
		})
	}
}

func TestGetGeoLocation(t *testing.T) {
	log := &mockLogger{}

	// Since we may not have the IP2Location database available in tests,
	// we'll just verify that the function returns empty string gracefully
	// when the database is not loaded

	// Test with empty IP
	country := GetGeoLocation("", log)
	assert.Equal(t, "", country)

	// Test with invalid IP
	country = GetGeoLocation("invalid-ip", log)
	assert.Equal(t, "", country)

	// Test with valid IP (but likely no database available in tests)
	// This should return empty string instead of panicking
	country = GetGeoLocation("8.8.8.8", log)
	// We just check it doesn't panic, the actual value depends on whether
	// the database is loaded
	assert.NotPanics(t, func() {
		GetGeoLocation("8.8.8.8", log)
	})
}
