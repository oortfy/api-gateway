package server

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"api-gateway/internal/config"
	"api-gateway/internal/handlers"
	"api-gateway/pkg/logger"
)

// GRPCServer represents the gRPC server component of the API gateway
type GRPCServer struct {
	config        *config.Config
	routes        *config.RouteConfig
	log           logger.Logger
	server        *grpc.Server
	handlers      map[string]*handlers.GRPCHandler
	mu            sync.RWMutex
	serviceRoutes map[string]*config.Route // map of full service names to route configs
	addr          string
}

// NewGRPCServer creates a new gRPC server
func NewGRPCServer(cfg *config.Config, routes *config.RouteConfig, log logger.Logger) *GRPCServer {
	// Create server options
	serverOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(cfg.GRPC.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(cfg.GRPC.MaxSendMsgSize),
	}

	// Create the gRPC server
	server := grpc.NewServer(serverOpts...)

	// Determine address for gRPC (default: same as HTTP but on port+1)
	addr := cfg.Server.Address
	if strings.Contains(addr, ":") {
		parts := strings.Split(addr, ":")
		if len(parts) == 2 {
			// Extract port number and increment by 1
			port := parts[1]
			newPort := ""
			if port != "" {
				portNum := 0
				fmt.Sscanf(port, "%d", &portNum)
				if portNum > 0 {
					newPort = fmt.Sprintf("%d", portNum+1)
				}
			}

			if newPort != "" {
				addr = fmt.Sprintf("%s:%s", parts[0], newPort)
			}
		}
	}

	return &GRPCServer{
		config:        cfg,
		routes:        routes,
		log:           log,
		server:        server,
		handlers:      make(map[string]*handlers.GRPCHandler),
		serviceRoutes: make(map[string]*config.Route),
		addr:          addr,
	}
}

// RegisterRoutes sets up the gRPC service handlers based on the route configuration
func (s *GRPCServer) RegisterRoutes() error {
	// Register UnknownServiceHandler to capture all incoming requests
	s.server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "gateway",
		HandlerType: nil,
		Methods:     []grpc.MethodDesc{},
		Streams:     []grpc.StreamDesc{},
		Metadata:    nil,
	}, s)

	// Enable reflection for development/debugging
	if s.config.GRPC.EnableReflection {
		reflection.Register(s.server)
	}

	// Store route configurations for each gRPC service for later lookup
	for i := range s.routes.Routes {
		route := &s.routes.Routes[i]

		// Only register routes where both protocols are gRPC
		if route.Protocol == config.ProtocolGRPC && route.EndpointsProtocol == config.ProtocolGRPC {
			// Extract service name from the path
			serviceName := strings.TrimSuffix(route.Path, "/*")

			// Create handler for this service
			handler, err := handlers.NewGRPCHandler(route, s.log)
			if err != nil {
				return fmt.Errorf("failed to create handler for %s: %w", serviceName, err)
			}

			// Store the handler
			s.handlers[serviceName] = handler
			s.serviceRoutes[serviceName] = route

			s.log.Info("Registered gRPC service",
				logger.String("service", serviceName),
				logger.String("upstream", route.Upstream),
			)
		}
	}

	return nil
}

// Start starts the gRPC server
func (s *GRPCServer) Start() error {
	// Register the routes
	if err := s.RegisterRoutes(); err != nil {
		return err
	}

	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	s.log.Info("Starting gRPC server", logger.String("address", s.addr))
	return s.server.Serve(lis)
}

// Stop stops the gRPC server
func (s *GRPCServer) Stop() {
	s.log.Info("Stopping gRPC server")
	s.server.GracefulStop()

	// Close all handlers
	for _, handler := range s.handlers {
		handler.Close()
	}
}

// UnaryHandler is the fallback handler for all unary RPC methods
func (s *GRPCServer) UnaryHandler(ctx context.Context, req interface{}) (interface{}, error) {
	// Extract full method name from the context
	fullMethod, ok := grpc.Method(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "method info not found in context")
	}

	// Split the full method name to get service and method
	parts := strings.Split(fullMethod, "/")
	if len(parts) != 3 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid method name format: %s", fullMethod)
	}

	serviceName := parts[1]
	methodName := parts[2]
	fullServiceMethod := fmt.Sprintf("%s/%s", serviceName, methodName)

	// Look up the appropriate handler
	s.mu.RLock()
	handler, exists := s.handlers[serviceName]
	route := s.serviceRoutes[serviceName]
	s.mu.RUnlock()

	if !exists || route == nil {
		s.log.Error("Service not found",
			logger.String("service", serviceName),
			logger.String("method", methodName),
		)
		return nil, status.Errorf(codes.Unimplemented, "service %s not found", serviceName)
	}

	// Check if request message is proto.Message
	requestMsg, ok := req.(proto.Message)
	if !ok {
		return nil, status.Error(codes.Internal, "request is not a proto.Message")
	}

	// Apply authentication middleware if required
	if route.Middlewares != nil && route.Middlewares.RequireAuth {
		// Get metadata from context
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "metadata required for authentication")
		}

		// Implement authentication logic here
		// This is a placeholder - actual implementation would vary
		authToken := ""
		if tokens := md.Get("authorization"); len(tokens) > 0 {
			authToken = tokens[0]
		}

		if authToken == "" {
			return nil, status.Error(codes.Unauthenticated, "authentication required")
		}

		// Validate token logic would go here
		// ...
	}

	// Forward the request to the backend service
	responseMsg, respMD, err := handler.ForwardUnary(ctx, fullServiceMethod, requestMsg)
	if err != nil {
		// Pass through the error code from the backend
		return nil, err
	}

	// Send back response metadata
	if err := grpc.SendHeader(ctx, respMD); err != nil {
		s.log.Error("Failed to send response headers", logger.Error(err))
	}

	return responseMsg, nil
}

// StreamHandler handles all streaming RPC methods (not implemented yet)
func (s *GRPCServer) StreamHandler(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	// This is a placeholder for streaming support
	// Full implementation would require more complex stream forwarding logic
	return status.Error(codes.Unimplemented, "streaming methods not supported by the gateway")
}

// GetGRPCConnection gets a connection to a backend gRPC service
func (s *GRPCServer) GetGRPCConnection(ctx context.Context, target string) (*grpc.ClientConn, error) {
	// Remove the grpc:// scheme if present
	if strings.HasPrefix(target, "grpc://") {
		target = strings.TrimPrefix(target, "grpc://")
	}

	// Create connection options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(s.config.GRPC.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(s.config.GRPC.MaxSendMsgSize),
		),
	}

	// Dial the target service
	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", target, err)
	}

	return conn, nil
}

// dynamicInvokeUnary invokes a unary RPC method dynamically
func dynamicInvokeUnary(ctx context.Context, conn *grpc.ClientConn, fullMethod string, input, output proto.Message) error {
	return conn.Invoke(ctx, fullMethod, input, output)
}

// findMethodDescriptor finds a method descriptor for a full method name
func findMethodDescriptor(fullMethod string) (protoreflect.MethodDescriptor, error) {
	// Parse the method name
	parts := strings.Split(fullMethod, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid method format: %s", fullMethod)
	}

	serviceName := parts[0]
	methodName := parts[1]

	// Find the service descriptor
	sd, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		return nil, fmt.Errorf("service not found: %s", serviceName)
	}

	service, ok := sd.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("not a service: %s", serviceName)
	}

	// Find the method
	method := service.Methods().ByName(protoreflect.Name(methodName))
	if method == nil {
		return nil, fmt.Errorf("method not found: %s", methodName)
	}

	return method, nil
}
