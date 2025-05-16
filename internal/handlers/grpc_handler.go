package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	"api-gateway/internal/config"
	"api-gateway/internal/proxy"
	"api-gateway/pkg/logger"
)

// GRPCHandler handles gRPC forwarding
type GRPCHandler struct {
	route     *config.Route
	grpcProxy *proxy.GRPCProxy
	logger    logger.Logger
	mu        sync.RWMutex
}

// NewGRPCHandler creates a new gRPC handler
func NewGRPCHandler(route *config.Route, log logger.Logger) (*GRPCHandler, error) {
	if route == nil {
		return nil, fmt.Errorf("route configuration is required")
	}

	// Create gRPC proxy with default connection settings
	grpcProxy := proxy.NewGRPCProxy(5*time.Minute, 100, log) // 5 min idle timeout, 100 max connections

	return &GRPCHandler{
		route:     route,
		grpcProxy: grpcProxy,
		logger:    log,
	}, nil
}

// ForwardUnary forwards a unary gRPC request to the upstream service
func (h *GRPCHandler) ForwardUnary(
	ctx context.Context,
	fullMethodName string,
	requestMessage proto.Message,
) (proto.Message, metadata.MD, error) {
	h.mu.RLock()
	target := h.route.Upstream
	h.mu.RUnlock()

	// If we have a load balancer, use it to select target
	if h.route.LoadBalancing != nil && len(h.route.LoadBalancing.Endpoints) > 0 {
		// Use first endpoint for now - later this would use load balancing logic
		target = h.route.LoadBalancing.Endpoints[0]
	}

	// Forward the gRPC request
	return h.grpcProxy.ForwardGRPC(ctx, fullMethodName, target, requestMessage)
}

// ServerStreamForwarder handles server streaming gRPC methods
func (h *GRPCHandler) ServerStreamForwarder(
	srv interface{},
	stream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	// This is a placeholder for server streaming support
	// Implementing full streaming support requires more complex code
	return fmt.Errorf("streaming not implemented for gRPC proxy yet")
}

// Close closes the gRPC handler and its resources
func (h *GRPCHandler) Close() error {
	h.grpcProxy.Close()
	return nil
}
