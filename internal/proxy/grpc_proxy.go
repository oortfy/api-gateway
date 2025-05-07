package proxy

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"api-gateway/internal/config"
)

// GRPCProxy handles gRPC proxy operations
type GRPCProxy struct {
	route       *config.Route
	connections sync.Map
	dialOptions []grpc.DialOption
}

// NewGRPCProxy creates a new gRPC proxy instance
func NewGRPCProxy(route *config.Route) (*GRPCProxy, error) {
	if route == nil {
		return nil, fmt.Errorf("route configuration is required")
	}

	// Default dial options
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024 * 1024 * 16)), // 16MB
	}

	return &GRPCProxy{
		route:       route,
		dialOptions: dialOpts,
	}, nil
}

// getConnection returns a gRPC connection for the given target
func (p *GRPCProxy) getConnection(target string) (*grpc.ClientConn, error) {
	if conn, ok := p.connections.Load(target); ok {
		return conn.(*grpc.ClientConn), nil
	}

	conn, err := grpc.Dial(target, p.dialOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial gRPC server: %w", err)
	}

	p.connections.Store(target, conn)
	return conn, nil
}

// ProxyGRPCToGRPC handles pure gRPC to gRPC proxying
func (p *GRPCProxy) ProxyGRPCToGRPC(ctx context.Context, fullMethodName string, req []byte) ([]byte, error) {
	conn, err := p.getConnection(p.route.Upstream)
	if err != nil {
		return nil, err
	}

	// Extract metadata from context
	md, _ := metadata.FromIncomingContext(ctx)
	outCtx := metadata.NewOutgoingContext(ctx, md.Copy())

	// Make the gRPC call
	var resp interface{}
	if err := conn.Invoke(outCtx, fullMethodName, req, &resp, grpc.CallContentSubtype("proto")); err != nil {
		return nil, err
	}

	// Convert response to bytes
	if respBytes, ok := resp.([]byte); ok {
		return respBytes, nil
	}

	// If response is a protobuf message, marshal it
	if msg, ok := resp.(proto.Message); ok {
		return proto.Marshal(msg)
	}

	return nil, fmt.Errorf("unexpected response type: %T", resp)
}

// ProxyHTTPToGRPC converts HTTP request to gRPC and forwards it
func (p *GRPCProxy) ProxyHTTPToGRPC(w http.ResponseWriter, r *http.Request) {
	// Extract method name from path
	methodName := strings.TrimPrefix(r.URL.Path, p.route.Path)
	if methodName == "" {
		http.Error(w, "method name not specified", http.StatusBadRequest)
		return
	}

	// Create gRPC context with headers
	ctx := r.Context()
	md := metadata.New(nil)
	for k, v := range r.Header {
		md.Set(k, v...)
	}
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Read request body
	var reqBody []byte
	if r.Body != nil {
		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("failed to parse form: %v", err), http.StatusBadRequest)
			return
		}
		reqBody = []byte(r.Form.Encode())
	}

	// Make gRPC call
	resp, err := p.ProxyGRPCToGRPC(ctx, methodName, reqBody)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			code := httpStatusFromGRPCCode(st.Code())
			http.Error(w, st.Message(), code)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert response to JSON
	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// ProxyGRPCToHTTP converts gRPC request to HTTP and forwards it
func (p *GRPCProxy) ProxyGRPCToHTTP(ctx context.Context, fullMethodName string, req []byte) ([]byte, error) {
	// Create HTTP client
	client := &http.Client{}

	// Convert gRPC metadata to HTTP headers
	md, _ := metadata.FromIncomingContext(ctx)
	headers := make(http.Header)
	for k, v := range md {
		headers[k] = v
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.route.Upstream, strings.NewReader(string(req)))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create HTTP request: %v", err)
	}
	httpReq.Header = headers

	// Make HTTP call
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Convert HTTP response to gRPC
	if resp.StatusCode != http.StatusOK {
		return nil, status.Errorf(httpCodeToGRPCCode(resp.StatusCode), "HTTP request failed with status: %d", resp.StatusCode)
	}

	// Read response body
	respBody := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			respBody = append(respBody, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	return respBody, nil
}

// Close closes all gRPC connections
func (p *GRPCProxy) Close() error {
	var lastErr error
	p.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*grpc.ClientConn); ok {
			if err := conn.Close(); err != nil {
				lastErr = err
			}
		}
		return true
	})
	return lastErr
}

// Helper functions for status code conversion
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
	case codes.Unauthenticated:
		return http.StatusUnauthorized
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

func httpCodeToGRPCCode(httpCode int) codes.Code {
	switch httpCode {
	case http.StatusOK:
		return codes.OK
	case http.StatusBadRequest:
		return codes.InvalidArgument
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusConflict:
		return codes.AlreadyExists
	case http.StatusTooManyRequests:
		return codes.ResourceExhausted
	case http.StatusNotImplemented:
		return codes.Unimplemented
	case http.StatusInternalServerError:
		return codes.Internal
	case http.StatusServiceUnavailable:
		return codes.Unavailable
	default:
		return codes.Unknown
	}
}
