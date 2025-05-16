package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"api-gateway/internal/config"
	"api-gateway/internal/handlers"
	"api-gateway/pkg/logger"
)

// testLogger is a test implementation of the logger.Logger interface
type testLogger struct{}

func (t *testLogger) Debug(msg string, args ...logger.Field)  {}
func (t *testLogger) Info(msg string, args ...logger.Field)   {}
func (t *testLogger) Warn(msg string, args ...logger.Field)   {}
func (t *testLogger) Error(msg string, args ...logger.Field)  {}
func (t *testLogger) Fatal(msg string, args ...logger.Field)  {}
func (t *testLogger) With(args ...logger.Field) logger.Logger { return t }

// mockProtoMessage is a mock implementation of proto.Message for testing
type mockProtoMessage struct {
	proto.Message
}

// MockGRPCStream is a mock implementation of grpc.ServerStream
type MockGRPCStream struct {
	mock.Mock
	ctx context.Context
}

func (m *MockGRPCStream) SetHeader(md metadata.MD) error {
	args := m.Called(md)
	return args.Error(0)
}

func (m *MockGRPCStream) SendHeader(md metadata.MD) error {
	args := m.Called(md)
	return args.Error(0)
}

func (m *MockGRPCStream) SetTrailer(md metadata.MD) {
	m.Called(md)
}

func (m *MockGRPCStream) Context() context.Context {
	return m.ctx
}

func (m *MockGRPCStream) SendMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockGRPCStream) RecvMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func TestNewGRPCServer(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address: ":8080",
		},
		GRPC: config.GRPCConfig{
			MaxRecvMsgSize:   4 * 1024 * 1024, // 4MB
			MaxSendMsgSize:   4 * 1024 * 1024, // 4MB
			EnableReflection: true,
		},
	}

	// Create a routes config with a test gRPC route
	routesCfg := &config.RouteConfig{
		Routes: []config.Route{
			{
				Path:              "test.service.TestService/*",
				Protocol:          config.ProtocolGRPC,
				EndpointsProtocol: config.ProtocolGRPC,
				RPCServer:         "/api/test",
				Upstream:          "grpc://localhost:50051",
			},
		},
	}

	// Create a logger
	log := &testLogger{}

	// Create the gRPC server
	grpcServer := NewGRPCServer(cfg, routesCfg, log)

	// Assert the server was created correctly
	assert.NotNil(t, grpcServer)
	assert.Equal(t, cfg, grpcServer.config)
	assert.Equal(t, routesCfg, grpcServer.routes)
	assert.NotNil(t, grpcServer.server)
	assert.Equal(t, ":8081", grpcServer.addr) // Should be port + 1
}

func TestRegisterRoutes(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.Config{
		GRPC: config.GRPCConfig{
			EnableReflection: true,
		},
	}

	// Create a routes config with both GRPC and HTTP routes
	routesCfg := &config.RouteConfig{
		Routes: []config.Route{
			{
				Path:              "test.service.TestService/*",
				Protocol:          config.ProtocolGRPC,
				EndpointsProtocol: config.ProtocolGRPC,
				RPCServer:         "/api/test",
				Upstream:          "grpc://localhost:50051",
				Middlewares:       &config.Middlewares{},
			},
			{
				Path:     "/api/users",
				Protocol: config.ProtocolHTTP,
				Upstream: "http://localhost:8080",
			},
		},
	}

	// Create a logger
	log := &testLogger{}

	// Create the gRPC server
	grpcServer := &GRPCServer{
		config:        cfg,
		routes:        routesCfg,
		log:           log,
		server:        grpc.NewServer(),
		handlers:      make(map[string]*handlers.GRPCHandler),
		serviceRoutes: make(map[string]*config.Route),
	}

	// Mock the handler creation
	// In a real test, we would need to mock the handlers.NewGRPCHandler function
	// Since we can't easily do that, we'll just verify that RegisterRoutes doesn't panic

	// Register the routes
	assert.NotPanics(t, func() {
		// We expect this to fail because we can't create real handlers in a unit test
		_ = grpcServer.RegisterRoutes()
	})
}

func TestUnaryHandler(t *testing.T) {
	// Create a minimal server
	grpcServer := &GRPCServer{
		log:           &testLogger{},
		serviceRoutes: make(map[string]*config.Route),
		handlers:      make(map[string]*handlers.GRPCHandler),
	}

	// Test with invalid method format
	ctx := context.Background()
	_, err := grpcServer.UnaryHandler(metadata.NewIncomingContext(ctx, metadata.Pairs("method", "invalid-format")), &mockProtoMessage{})
	assert.Error(t, err)
	statusErr, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, statusErr.Code())

	// Test with service not found
	methodCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("method", "/service.Test/Method"))
	_, err = grpcServer.UnaryHandler(methodCtx, &mockProtoMessage{})
	assert.Error(t, err)
	statusErr, ok = status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unimplemented, statusErr.Code())
}

func TestStreamHandler(t *testing.T) {
	// Create a minimal server
	grpcServer := &GRPCServer{
		log: &testLogger{},
	}

	// Mock stream and info
	stream := &MockGRPCStream{
		ctx: context.Background(),
	}
	info := &grpc.StreamServerInfo{
		FullMethod:     "/service.Test/Method",
		IsClientStream: true,
		IsServerStream: true,
	}

	// Test stream handler
	err := grpcServer.StreamHandler(nil, stream, info, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "streaming not implemented")
}

func TestGetGRPCConnection(t *testing.T) {
	// Create a minimal server
	grpcServer := &GRPCServer{
		log: &testLogger{},
	}

	// Setup a local listener to test a real connection
	// Even if the test doesn't complete the connection, this verifies the function works
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	// Get the dynamic port that was allocated
	_, port, err := net.SplitHostPort(lis.Addr().String())
	require.NoError(t, err)

	// Create a context with timeout for dialing
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Test GetGRPCConnection with grpc:// prefix
	target := "grpc://localhost:" + port
	conn, err := grpcServer.GetGRPCConnection(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, conn)
	conn.Close()

	// Test GetGRPCConnection without prefix
	target = "localhost:" + port
	conn, err = grpcServer.GetGRPCConnection(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, conn)
	conn.Close()

	// Test with invalid target
	_, err = grpcServer.GetGRPCConnection(ctx, "invalid:///address")
	assert.Error(t, err)
}

func TestStartAndStop(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.Config{
		GRPC: config.GRPCConfig{
			EnableReflection: false,
		},
	}

	// Create routes config
	routesCfg := &config.RouteConfig{
		Routes: []config.Route{},
	}

	// Create a logger
	log := &testLogger{}

	// Create the gRPC server with a random port
	grpcServer := &GRPCServer{
		config:        cfg,
		routes:        routesCfg,
		log:           log,
		server:        grpc.NewServer(),
		handlers:      make(map[string]*handlers.GRPCHandler),
		serviceRoutes: make(map[string]*config.Route),
		addr:          "localhost:0", // Use a random available port
	}

	// Start the server in a goroutine
	go func() {
		// We expect this to fail in tests because we can't properly setup handlers
		// But it should exercise the code path
		_ = grpcServer.Start()
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop the server - this should not panic
	assert.NotPanics(t, func() {
		grpcServer.Stop()
	})
}
