package proxy

import (
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMockLogger creates a test logger
func setupMockLogger() logger.Logger {
	return &mockLogger{}
}

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

func TestNewHTTPProxy(t *testing.T) {
	cfg := &config.Config{}
	routes := &config.RouteConfig{}
	log := &mockLogger{}

	proxy := NewHTTPProxy(cfg, routes, log)

	assert.NotNil(t, proxy)
	assert.Equal(t, cfg, proxy.config)
	assert.Equal(t, routes, proxy.routes)
	assert.Equal(t, log, proxy.log)
	assert.NotNil(t, proxy.circuitBreakers)
}

func TestProxyRequestBasic(t *testing.T) {
	// Create a test upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK from upstream"))
	}))
	defer upstream.Close()

	// Create a mock route
	route := config.Route{
		Path:        "/api/test",
		Upstream:    upstream.URL,
		Methods:     []string{"GET"},
		StripPrefix: true,
		Middlewares: &config.Middlewares{}, // Add this to avoid nil pointer
	}

	// Create the HTTP proxy with proper etcd config
	cfg := &config.Config{
		Etcd: config.EtcdConfig{
			Hosts: "localhost:2379", // Provide a default value
		},
	}
	routes := &config.RouteConfig{
		Routes: []config.Route{route}, // Add route to the config
	}
	log := &mockLogger{}
	proxy := NewHTTPProxy(cfg, routes, log)

	// Create a handler using the proxy
	handler := proxy.ProxyRequest(route)
	require.NotNil(t, handler)

	// Create a test request
	req := httptest.NewRequest("GET", "http://example.com/api/test/resource", nil)
	req.Header.Set("X-Custom-Header", "custom-value")

	// Record the response
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Check the response
	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "test-value", resp.Header.Get("X-Test-Header"))
}

func TestProxyRequestWithStripPrefix(t *testing.T) {
	// Create a test server that echoes back the request path
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Requested path: " + r.URL.Path))
	}))
	defer upstream.Close()

	testCases := []struct {
		name         string
		stripPrefix  bool
		requestPath  string
		expectedPath string
	}{
		{
			name:         "with strip prefix",
			stripPrefix:  true,
			requestPath:  "/api/test/resource",
			expectedPath: "/resource", // /api/test is stripped
		},
		{
			name:         "without strip prefix",
			stripPrefix:  false,
			requestPath:  "/api/test/resource",
			expectedPath: "/api/test/resource", // full path preserved
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock route
			route := config.Route{
				Path:        "/api/test",
				Upstream:    upstream.URL,
				Methods:     []string{"GET"},
				StripPrefix: tc.stripPrefix,
				Middlewares: &config.Middlewares{}, // Add this to avoid nil pointer
			}

			// Create the HTTP proxy with proper config
			cfg := &config.Config{
				Etcd: config.EtcdConfig{
					Hosts: "localhost:2379", // Provide a default value
				},
			}
			routes := &config.RouteConfig{
				Routes: []config.Route{route}, // Add the route to config
			}
			log := &mockLogger{}
			proxy := NewHTTPProxy(cfg, routes, log)

			// Create a handler using the proxy
			handler := proxy.ProxyRequest(route)
			require.NotNil(t, handler)

			// Create a test request
			req := httptest.NewRequest("GET", "http://example.com"+tc.requestPath, nil)

			// Record the response
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// Check the response
			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			// Read the body
			body := make([]byte, 1024)
			n, _ := resp.Body.Read(body)
			bodyStr := string(body[:n])

			// Check if the path was correctly processed
			expectedResponse := "Requested path: " + tc.expectedPath
			assert.Contains(t, bodyStr, expectedResponse)
		})
	}
}

func TestProxyRequestWithHeaders(t *testing.T) {
	// Create a test server that echoes back headers
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the headers we're interested in
		w.Header().Set("X-Received-XFF", r.Header.Get("X-Forwarded-For"))
		w.Header().Set("X-Received-Real-IP", r.Header.Get("X-Real-IP"))
		w.Header().Set("X-Received-Auth", r.Header.Get("Authorization"))
		w.Header().Set("X-Received-API-Key", r.Header.Get("x-api-key"))
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	// Create a mock route
	route := config.Route{
		Path:        "/api/test",
		Upstream:    upstream.URL,
		Methods:     []string{"GET"},
		StripPrefix: true,
		Middlewares: &config.Middlewares{}, // Add this to avoid nil pointer
	}

	// Create the HTTP proxy
	cfg := &config.Config{
		Etcd: config.EtcdConfig{
			Hosts: "localhost:2379", // Provide a default value
		},
	}
	routes := &config.RouteConfig{
		Routes: []config.Route{route}, // Add the route to config
	}
	log := &mockLogger{}
	proxy := NewHTTPProxy(cfg, routes, log)

	// Create a handler using the proxy
	handler := proxy.ProxyRequest(route)
	require.NotNil(t, handler)

	// Test with query parameters for auth
	t.Run("with query parameters", func(t *testing.T) {
		// Create a test request with query parameters
		req := httptest.NewRequest("GET", "http://example.com/api/test/resource?token=jwt-token&api_key=api-key-value", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1")

		// Record the response
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Check the response
		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("X-Received-XFF"), "192.168.1.1")
		assert.Equal(t, "Bearer jwt-token", resp.Header.Get("X-Received-Auth"))
		assert.Equal(t, "api-key-value", resp.Header.Get("X-Received-API-Key"))
	})

	// Test with headers for auth
	t.Run("with auth headers", func(t *testing.T) {
		// Create a test request with auth headers
		req := httptest.NewRequest("GET", "http://example.com/api/test/resource", nil)
		req.Header.Set("Authorization", "Bearer header-jwt-token")
		req.Header.Set("x-api-key", "header-api-key")
		req.Header.Set("X-Real-IP", "10.0.0.1")

		// Record the response
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Check the response
		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "10.0.0.1", resp.Header.Get("X-Received-Real-IP"))
		assert.Equal(t, "Bearer header-jwt-token", resp.Header.Get("X-Received-Auth"))
		assert.Equal(t, "header-api-key", resp.Header.Get("X-Received-API-Key"))
	})
}

// TestHTTPProxy_parseURLs tests the parseURLs function
func TestHTTPProxy_parseURLs(t *testing.T) {
	// Create a mock logger
	mockLogger := setupMockLogger()

	// Create a proxy
	proxy := NewHTTPProxy(&config.Config{}, &config.RouteConfig{}, mockLogger)

	// Test cases
	testCases := []struct {
		name          string
		protocol      string
		addresses     []string
		expectedCount int
		expectError   bool
	}{
		{
			name:          "empty_addresses",
			protocol:      "http",
			addresses:     []string{},
			expectedCount: 0,
			expectError:   false,
		},
		{
			name:          "valid_http_addresses",
			protocol:      "http",
			addresses:     []string{"localhost:8080", "127.0.0.1:8081"},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "valid_https_addresses",
			protocol:      "https",
			addresses:     []string{"localhost:8443", "127.0.0.1:8444"},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "addresses_with_protocol",
			protocol:      "http",
			addresses:     []string{"http://localhost:8080", "https://127.0.0.1:8443"},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "invalid_addresses",
			protocol:      "http",
			addresses:     []string{"://invalid"},
			expectedCount: 0,
			expectError:   true,
		},
		{
			name:          "mixed_valid_and_invalid",
			protocol:      "http",
			addresses:     []string{"localhost:8080", "://invalid"},
			expectedCount: 0,
			expectError:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			urls, err := proxy.parseURLs(tc.protocol, tc.addresses)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedCount, len(urls))

				// Check that the protocol was applied correctly
				for _, u := range urls {
					if strings.HasPrefix(tc.addresses[0], "http://") || strings.HasPrefix(tc.addresses[0], "https://") {
						// If the address already specifies a protocol, it should be preserved
						continue
					}
					assert.Equal(t, tc.protocol, u.Scheme)
				}
			}
		})
	}
}
