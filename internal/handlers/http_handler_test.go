package handlers

import (
	"api-gateway/internal/config"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPHandler(t *testing.T) {
	testCases := []struct {
		name        string
		route       *config.Route
		expectError bool
	}{
		{
			name:        "nil_route",
			route:       nil,
			expectError: true,
		},
		{
			name: "invalid_upstream_url",
			route: &config.Route{
				Path:     "/test",
				Upstream: "://invalid-url",
				Protocol: config.ProtocolHTTP,
			},
			expectError: true,
		},
		{
			name: "valid_route",
			route: &config.Route{
				Path:     "/test",
				Upstream: "http://valid-upstream.example.com",
				Protocol: config.ProtocolHTTP,
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := NewHTTPHandler(tc.route)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, handler)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, handler)
				assert.Equal(t, tc.route, handler.route)
				assert.NotNil(t, handler.proxy)
			}
		})
	}
}

func TestHTTPHandler_ServeHTTP(t *testing.T) {
	// Create a test upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo request information for testing
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Request-Path", r.URL.Path)
		w.Header().Set("X-Request-Host", r.Host)

		if r.URL.Path == "/error" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success: " + r.URL.Path))
	}))
	defer upstream.Close()

	testCases := []struct {
		name             string
		route            *config.Route
		requestPath      string
		expectedStatus   int
		expectedResponse string
		expectedHeaders  map[string]string
	}{
		{
			name: "protocol_mismatch",
			route: &config.Route{
				Path:     "/api",
				Upstream: upstream.URL,
				Protocol: config.ProtocolGRPC,
			},
			requestPath:      "/api/test",
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: "Protocol mismatch\n",
		},
		{
			name: "strip_prefix",
			route: &config.Route{
				Path:        "/api",
				Upstream:    upstream.URL,
				Protocol:    config.ProtocolHTTP,
				StripPrefix: true,
			},
			requestPath:      "/api/test",
			expectedStatus:   http.StatusOK,
			expectedResponse: "Success: /test",
			expectedHeaders: map[string]string{
				"X-Request-Path": "/test",
			},
		},
		{
			name: "preserve_prefix",
			route: &config.Route{
				Path:        "/api",
				Upstream:    upstream.URL,
				Protocol:    config.ProtocolHTTP,
				StripPrefix: false,
			},
			requestPath:      "/api/test",
			expectedStatus:   http.StatusOK,
			expectedResponse: "Success: /api/test",
			expectedHeaders: map[string]string{
				"X-Request-Path": "/api/test",
			},
		},
		{
			name: "header_transform",
			route: &config.Route{
				Path:        "/api",
				Upstream:    upstream.URL,
				Protocol:    config.ProtocolHTTP,
				StripPrefix: true,
				Middlewares: &config.Middlewares{
					HeaderTransform: &config.HeaderTransform{
						Request:  map[string]string{"X-Added-Header": "test-value"},
						Response: map[string]string{"X-Response-Header": "response-value"},
						Remove:   []string{"X-Request-Path"},
					},
				},
			},
			requestPath:      "/api/test",
			expectedStatus:   http.StatusOK,
			expectedResponse: "Success: /test",
			expectedHeaders: map[string]string{
				"X-Response-Header": "response-value",
			},
		},
		{
			name: "error_handling_default",
			route: &config.Route{
				Path:        "/api",
				Upstream:    upstream.URL,
				Protocol:    config.ProtocolHTTP,
				StripPrefix: true,
				ErrorHandling: &config.ErrorHandling{
					DefaultMessage: "Custom error message",
				},
			},
			requestPath:      "/api/error",
			expectedStatus:   http.StatusInternalServerError,
			expectedResponse: "Custom error message",
		},
		{
			name: "error_handling_status_code",
			route: &config.Route{
				Path:        "/api",
				Upstream:    upstream.URL,
				Protocol:    config.ProtocolHTTP,
				StripPrefix: true,
				ErrorHandling: &config.ErrorHandling{
					DefaultMessage: "Default error",
					StatusCodes:    map[int]string{http.StatusInternalServerError: "Custom 500 error message"},
				},
			},
			requestPath:      "/api/error",
			expectedStatus:   http.StatusInternalServerError,
			expectedResponse: "Custom 500 error message",
		},
		{
			name: "url_rewrite",
			route: &config.Route{
				Path:        "/api",
				Upstream:    upstream.URL,
				Protocol:    config.ProtocolHTTP,
				StripPrefix: false,
				Middlewares: &config.Middlewares{
					URLRewrite: &config.URLRewrite{
						Patterns: []config.URLRewritePattern{
							{
								Match:       "/api/old",
								Replacement: "/api/new",
							},
						},
					},
				},
			},
			requestPath:      "/api/old",
			expectedStatus:   http.StatusOK,
			expectedResponse: "Success: /api/new",
			expectedHeaders: map[string]string{
				"X-Request-Path": "/api/new",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := NewHTTPHandler(tc.route)
			require.NoError(t, err)
			require.NotNil(t, handler)

			req := httptest.NewRequest("GET", tc.requestPath, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tc.expectedStatus, rec.Code)

			respBody, err := io.ReadAll(rec.Body)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResponse, string(respBody))

			for k, v := range tc.expectedHeaders {
				assert.Equal(t, v, rec.Header().Get(k))
			}
		})
	}
}

// TestHTTPHandler_ErrorHandler tests the error handler function
func TestHTTPHandler_ErrorHandler(t *testing.T) {
	testCases := []struct {
		name           string
		errorHandling  *config.ErrorHandling
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "default_error",
			errorHandling:  nil,
			expectedStatus: http.StatusBadGateway,
			expectedBody:   "test error\n",
		},
		{
			name: "custom_default_message",
			errorHandling: &config.ErrorHandling{
				DefaultMessage: "Custom default error",
			},
			expectedStatus: http.StatusBadGateway,
			expectedBody:   "Custom default error\n",
		},
		{
			name: "custom_status_message",
			errorHandling: &config.ErrorHandling{
				DefaultMessage: "Default error",
				StatusCodes:    map[int]string{http.StatusBadGateway: "Custom 502 error"},
			},
			expectedStatus: http.StatusBadGateway,
			expectedBody:   "Custom 502 error\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			route := &config.Route{
				Path:          "/test",
				Upstream:      "http://example.com",
				Protocol:      config.ProtocolHTTP,
				ErrorHandling: tc.errorHandling,
			}

			handler, err := NewHTTPHandler(route)
			require.NoError(t, err)

			// Create a request and response recorder
			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()

			// Call the error handler directly
			testError := "test error"
			handler.proxy.ErrorHandler(rec, req, errors.New(testError))

			assert.Equal(t, tc.expectedStatus, rec.Code)
			assert.Equal(t, tc.expectedBody, rec.Body.String())
		})
	}
}
