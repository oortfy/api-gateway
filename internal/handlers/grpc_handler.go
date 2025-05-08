package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

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

	grpcProxy, err := proxy.NewGRPCProxy(route)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC proxy: %w", err)
	}

	return &GRPCHandler{
		route:     route,
		grpcProxy: grpcProxy,
	}, nil
}

// ServeHTTP implements the http.Handler interface
func (h *GRPCHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if this is a gRPC request
	if isGRPCRequest(r) {
		h.handleGRPCRequest(w, r)
		return
	}

	// Handle HTTP request
	if h.route.EndpointsProtocol == config.ProtocolGRPC {
		h.grpcProxy.ProxyHTTPToGRPC(w, r)
		return
	}

	http.Error(w, "unsupported protocol", http.StatusBadRequest)
}

// handleGRPCRequest handles incoming gRPC requests
func (h *GRPCHandler) handleGRPCRequest(w http.ResponseWriter, r *http.Request) {
	// Validate gRPC request
	if r.ProtoMajor != 2 {
		http.Error(w, "gRPC requires HTTP/2", http.StatusBadRequest)
		return
	}

	// Extract full method name from the path
	fullMethodName := strings.TrimPrefix(r.URL.Path, h.route.Path)
	if fullMethodName == "" {
		http.Error(w, "method name not specified", http.StatusBadRequest)
		return
	}

	// Create context with metadata
	ctx := r.Context()
	md := extractMetadata(r)
	ctx = metadata.NewIncomingContext(ctx, md)

	// Handle based on endpoint protocol
	switch h.route.EndpointsProtocol {
	case config.ProtocolHTTP:
		// gRPC to HTTP conversion
		resp, err := h.grpcProxy.ProxyGRPCToHTTP(ctx, fullMethodName, nil) // TODO: Read request body
		if err != nil {
			handleError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/grpc")
		w.Write(resp)

	case config.ProtocolGRPC, "":
		// Pure gRPC proxy
		resp, err := h.grpcProxy.ProxyGRPCToGRPC(ctx, fullMethodName, nil) // TODO: Read request body
		if err != nil {
			handleError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/grpc")
		w.Write(resp)

	default:
		http.Error(w, "unsupported protocol", http.StatusBadRequest)
	}
}

// Close closes the gRPC handler and its resources
func (h *GRPCHandler) Close() error {
	return h.grpcProxy.Close()
}

// Helper functions

func isGRPCRequest(r *http.Request) bool {
	return r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc")
}

func extractMetadata(r *http.Request) metadata.MD {
	md := metadata.MD{}
	for k, v := range r.Header {
		k = strings.ToLower(k)
		if isValidMetadataHeader(k) {
			md[k] = v
		}
	}
	return md
}

func isValidMetadataHeader(header string) bool {
	// List of headers that should not be propagated
	excluded := map[string]bool{
		"host":              true,
		"content-length":    true,
		"transfer-encoding": true,
		"connection":        true,
		"upgrade":           true,
	}
	return !excluded[header]
}

func handleError(w http.ResponseWriter, err error) {
	if st, ok := status.FromError(err); ok {
		w.Header().Set("Content-Type", "application/grpc")
		w.WriteHeader(httpStatusFromGRPCCode(st.Code()))
		// TODO: Write proper gRPC error response
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func httpStatusFromGRPCCode(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return http.StatusRequestTimeout
	case codes.Unknown:
		return http.StatusInternalServerError
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed
	case codes.Aborted:
		return http.StatusConflict
	case codes.OutOfRange:
		return http.StatusRequestedRangeNotSatisfiable
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Internal:
		return http.StatusInternalServerError
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DataLoss:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}
