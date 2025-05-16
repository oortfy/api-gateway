package proxy

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	grpcpool "api-gateway/pkg/grpc"
	"api-gateway/pkg/logger"
)

// GRPCProxy handles gRPC proxy operations
type GRPCProxy struct {
	pool        *grpcpool.ClientPool
	methodCache sync.Map // cache for method descriptors
	logger      logger.Logger
}

// Add a noopLogger type for use when no logger is provided
type noopLogger struct{}

func (n *noopLogger) Debug(msg string, args ...logger.Field)  {}
func (n *noopLogger) Info(msg string, args ...logger.Field)   {}
func (n *noopLogger) Warn(msg string, args ...logger.Field)   {}
func (n *noopLogger) Error(msg string, args ...logger.Field)  {}
func (n *noopLogger) Fatal(msg string, args ...logger.Field)  {}
func (n *noopLogger) With(args ...logger.Field) logger.Logger { return n }

// NewGRPCProxy creates a new gRPC proxy instance
func NewGRPCProxy(maxIdle time.Duration, maxConns int, log logger.Logger) *GRPCProxy {
	// Use noopLogger if no logger is provided
	if log == nil {
		log = &noopLogger{}
	}

	return &GRPCProxy{
		pool: grpcpool.NewClientPool(grpcpool.PoolConfig{
			MaxIdle:       maxIdle,
			MaxConns:      maxConns,
			HealthCheck:   true,
			CheckInterval: 30 * time.Second,
		}),
		logger: log,
	}
}

// ForwardGRPC handles direct gRPC to gRPC forwarding
func (p *GRPCProxy) ForwardGRPC(
	ctx context.Context,
	fullMethodName string,
	target string,
	requestMessage proto.Message,
) (proto.Message, metadata.MD, error) {
	// Get connection from pool
	conn, err := p.pool.GetConn(ctx, target)
	if err != nil {
		p.logger.Error("Failed to connect to gRPC service",
			logger.String("target", target),
			logger.Error(err),
		)
		return nil, nil, status.Errorf(codes.Unavailable, "failed to connect to backend: %v", err)
	}
	defer p.pool.ReleaseConn(target)

	// Get method descriptor from cache or create new
	methodDesc, err := p.getMethodDescriptor(fullMethodName)
	if err != nil {
		p.logger.Error("Invalid gRPC method",
			logger.String("method", fullMethodName),
			logger.Error(err),
		)
		return nil, nil, status.Errorf(codes.InvalidArgument, "invalid gRPC method: %v", err)
	}

	// Create output message
	outputMsg := dynamicMessage(methodDesc.Output())
	if outputMsg == nil {
		p.logger.Error("Failed to create output message",
			logger.String("method", fullMethodName),
		)
		return nil, nil, status.Error(codes.Internal, "failed to create output message")
	}

	// Make gRPC call with metadata
	var header metadata.MD
	err = conn.Invoke(ctx, fullMethodName, requestMessage, outputMsg, grpc.Header(&header))
	if err != nil {
		p.logger.Error("gRPC call failed",
			logger.String("method", fullMethodName),
			logger.String("target", target),
			logger.Error(err),
		)
		return nil, nil, err // preserve original gRPC error
	}

	return outputMsg, header, nil
}

// dynamicMessage creates a new dynamic proto message from a descriptor
func dynamicMessage(desc protoreflect.MessageDescriptor) proto.Message {
	msgType, err := protoregistry.GlobalTypes.FindMessageByName(desc.FullName())
	if err != nil {
		return nil
	}
	return msgType.New().Interface()
}

func (p *GRPCProxy) getMethodDescriptor(fullMethodName string) (protoreflect.MethodDescriptor, error) {
	// Check cache first
	if cached, ok := p.methodCache.Load(fullMethodName); ok {
		return cached.(protoreflect.MethodDescriptor), nil
	}

	// Parse service and method names
	parts := strings.Split(fullMethodName, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid method name format")
	}
	serviceName := parts[0]
	methodName := parts[1]

	// Get service descriptor
	sd, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		return nil, err
	}
	service := sd.(protoreflect.ServiceDescriptor)

	// Get method descriptor
	method := service.Methods().ByName(protoreflect.Name(methodName))
	if method == nil {
		return nil, fmt.Errorf("method %s not found", methodName)
	}

	// Cache the descriptor
	p.methodCache.Store(fullMethodName, method)
	return method, nil
}

// Close closes the gRPC connection pool
func (p *GRPCProxy) Close() {
	p.pool.Close()
}
