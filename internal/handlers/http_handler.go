package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"api-gateway/internal/config"
)

// HTTPHandler handles HTTP requests
type HTTPHandler struct {
	route *config.Route
	proxy *httputil.ReverseProxy
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler(route *config.Route) (*HTTPHandler, error) {
	if route == nil {
		return nil, fmt.Errorf("route configuration is required")
	}

	// Parse upstream URL
	target, err := url.Parse(route.Upstream)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Customize proxy behavior
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Preserve the original Host header if specified
		if !route.StripPrefix {
			req.Host = target.Host
		}

		// Handle path rewriting
		if route.Middlewares != nil && route.Middlewares.URLRewrite != nil {
			for _, pattern := range route.Middlewares.URLRewrite.Patterns {
				if strings.HasPrefix(req.URL.Path, pattern.Match) {
					req.URL.Path = strings.Replace(req.URL.Path, pattern.Match, pattern.Replacement, 1)
					break
				}
			}
		}
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		// Handle error responses
		if resp.StatusCode >= 500 {
			if route.ErrorHandling != nil {
				if customMsg, ok := route.ErrorHandling.StatusCodes[resp.StatusCode]; ok {
					resp.Body = io.NopCloser(strings.NewReader(customMsg))
					resp.ContentLength = int64(len(customMsg))
					return nil
				} else if route.ErrorHandling.DefaultMessage != "" {
					msg := route.ErrorHandling.DefaultMessage
					resp.Body = io.NopCloser(strings.NewReader(msg))
					resp.ContentLength = int64(len(msg))
					return nil
				}
			}
		}

		// Apply response header transformations
		if route.Middlewares != nil && route.Middlewares.HeaderTransform != nil {
			// Add/modify response headers
			for k, v := range route.Middlewares.HeaderTransform.Response {
				resp.Header.Set(k, v)
			}

			// Remove specified headers
			for _, header := range route.Middlewares.HeaderTransform.Remove {
				resp.Header.Del(header)
			}
		}

		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		code := http.StatusBadGateway
		msg := err.Error()

		if route.ErrorHandling != nil {
			if customMsg, ok := route.ErrorHandling.StatusCodes[code]; ok {
				msg = customMsg
			} else if route.ErrorHandling.DefaultMessage != "" {
				msg = route.ErrorHandling.DefaultMessage
			}
		}

		http.Error(w, msg, code)
	}

	return &HTTPHandler{
		route: route,
		proxy: proxy,
	}, nil
}

// ServeHTTP implements the http.Handler interface
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Ensure the request protocol matches
	if h.route.Protocol != config.ProtocolHTTP {
		http.Error(w, "Protocol mismatch", http.StatusBadRequest)
		return
	}

	// Strip prefix if configured
	if h.route.StripPrefix {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, h.route.Path)
		if !strings.HasPrefix(r.URL.Path, "/") {
			r.URL.Path = "/" + r.URL.Path
		}
	}

	// Apply header transformations
	if h.route.Middlewares != nil && h.route.Middlewares.HeaderTransform != nil {
		// Add/modify request headers
		for k, v := range h.route.Middlewares.HeaderTransform.Request {
			r.Header.Set(k, v)
		}

		// Remove specified headers
		for _, header := range h.route.Middlewares.HeaderTransform.Remove {
			r.Header.Del(header)
		}
	}

	// Forward the request
	h.proxy.ServeHTTP(w, r)
}
