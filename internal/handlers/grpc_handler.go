package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/config"
	"api-gateway/internal/proxy"
)

// GRPCHandler handles gRPC requests
type GRPCHandler struct {
	route     *config.Route
	grpcProxy *proxy.GRPCProxy
	mu        sync.RWMutex
}

// NewGRPCHandler creates a new gRPC handler
func NewGRPCHandler(route *config.Route) (*GRPCHandler, error) {
	if route == nil {
		return nil, fmt.Errorf("route configuration is required")
	}

	// Create gRPC proxy with default connection settings
	grpcProxy := proxy.NewGRPCProxy(5*time.Minute, 100) // 5 min idle timeout, 100 max connections

	return &GRPCHandler{
		route:     route,
		grpcProxy: grpcProxy,
	}, nil
}

// ServeHTTP implements the http.Handler interface
func (h *GRPCHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Create a new gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = r

	// Extract full method name from the path
	fullMethodName := strings.TrimPrefix(r.URL.Path, h.route.Path)
	if fullMethodName == "" {
		http.Error(w, "method name not specified", http.StatusBadRequest)
		return
	}

	// Handle the request using our optimized proxy
	h.grpcProxy.ProxyHandler(c, h.route.Upstream, fullMethodName)
}

// Close closes the gRPC handler and its resources
func (h *GRPCHandler) Close() error {
	h.grpcProxy.Close()
	return nil
}
