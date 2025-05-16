package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
)

// mockLogger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, args ...logger.Field)  {}
func (m *mockLogger) Info(msg string, args ...logger.Field)   {}
func (m *mockLogger) Warn(msg string, args ...logger.Field)   {}
func (m *mockLogger) Error(msg string, args ...logger.Field)  {}
func (m *mockLogger) Fatal(msg string, args ...logger.Field)  {}
func (m *mockLogger) With(args ...logger.Field) logger.Logger { return m }

// mockProtoMessage is a mock implementation of proto.Message for testing
type mockProtoMessage struct {
	proto.Message
}

func TestNewGRPCHandler(t *testing.T) {
	// Create a route configuration
	route := &config.Route{
		Path:              "test.service.TestService/*",
		Protocol:          config.ProtocolGRPC,
		EndpointsProtocol: config.ProtocolGRPC,
		RPCServer:         "/api/test",
		Upstream:          "grpc://localhost:50051",
		Middlewares:       &config.Middlewares{},
	}

	// Create a logger
	log := &mockLogger{}

	// Create a handler
	handler, err := NewGRPCHandler(route, log)

	// Assert the handler was created successfully
	require.NoError(t, err)
	assert.NotNil(t, handler)
	assert.Equal(t, route, handler.route)
	assert.NotNil(t, handler.grpcProxy)
	assert.Equal(t, log, handler.logger)
}

func TestNewGRPCHandlerWithNilRoute(t *testing.T) {
	// Create a logger
	log := &mockLogger{}

	// Create a handler with nil route
	handler, err := NewGRPCHandler(nil, log)

	// Assert an error was returned
	assert.Error(t, err)
	assert.Nil(t, handler)
	assert.Contains(t, err.Error(), "required")
}

func TestForwardUnary(t *testing.T) {
	// This test is a bit complex to implement fully since it requires mocking the gRPC proxy
	// We'll test the basic structure and error handling, but not the actual forwarding

	// Create a route configuration with load balancing
	route := &config.Route{
		Path:              "test.service.TestService/*",
		Protocol:          config.ProtocolGRPC,
		EndpointsProtocol: config.ProtocolGRPC,
		RPCServer:         "/api/test",
		Upstream:          "grpc://localhost:50051",
		Middlewares:       &config.Middlewares{},
		LoadBalancing: &config.LoadBalancingConfig{
			Method: "round_robin",
			Endpoints: []string{
				"grpc://localhost:50052",
				"grpc://localhost:50053",
			},
		},
	}

	// Create a logger
	log := &mockLogger{}

	// Create a handler
	handler, err := NewGRPCHandler(route, log)
	require.NoError(t, err)

	// Create a mock request message
	requestMsg := &mockProtoMessage{}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Forward the request (this will fail since there's no actual server)
	responseMsg, respMD, err := handler.ForwardUnary(ctx, "test.service.TestService/TestMethod", requestMsg)

	// We expect an error since there's no actual server to connect to
	assert.Error(t, err)
	assert.Nil(t, responseMsg)
	assert.Nil(t, respMD)
}

func TestClose(t *testing.T) {
	// Create a route configuration
	route := &config.Route{
		Path:              "test.service.TestService/*",
		Protocol:          config.ProtocolGRPC,
		EndpointsProtocol: config.ProtocolGRPC,
		RPCServer:         "/api/test",
		Upstream:          "grpc://localhost:50051",
		Middlewares:       &config.Middlewares{},
	}

	// Create a logger
	log := &mockLogger{}

	// Create a handler
	handler, err := NewGRPCHandler(route, log)
	require.NoError(t, err)

	// Close the handler
	err = handler.Close()

	// No error should be returned
	assert.NoError(t, err)
}
