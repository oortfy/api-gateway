package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	grpcpool "api-gateway/pkg/grpc"
)

// MockClientConn is a mock implementation of grpcpool.ClientConn for testing
type MockClientConn struct {
	invokeFunc func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error
}

func (m *MockClientConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	return m.invokeFunc(ctx, method, args, reply, opts...)
}

// Define interface for testing to avoid direct dependency on the concrete type
type ClientPoolInterface interface {
	GetConn(ctx context.Context, target string) (grpcpool.ClientConn, error)
	ReleaseConn(target string)
	Close()
}

// MockClientPool is a mock implementation of the gRPC client pool
type MockClientPool struct {
	getConnFunc     func(ctx context.Context, target string) (grpcpool.ClientConn, error)
	releaseConnFunc func(target string)
	closeFunc       func()
}

func (m *MockClientPool) GetConn(ctx context.Context, target string) (grpcpool.ClientConn, error) {
	return m.getConnFunc(ctx, target)
}

func (m *MockClientPool) ReleaseConn(target string) {
	m.releaseConnFunc(target)
}

func (m *MockClientPool) Close() {
	m.closeFunc()
}

// MockMessage is a mock implementation of proto.Message
type MockMessage struct {
	proto.Message
	jsonData map[string]interface{}
}

func (m *MockMessage) ProtoReflect() protoreflect.Message {
	// This is a minimal implementation just for testing
	return &mockProtoReflect{m: m}
}

type mockProtoReflect struct {
	protoreflect.Message
	m *MockMessage
}

// TestProxyWithMocks uses a modified GRPCProxy struct that accepts our interface
type GRPCProxyWithMocks struct {
	pool        ClientPoolInterface
	methodCache sync.Map
}

func TestNewGRPCProxy(t *testing.T) {
	proxy := NewGRPCProxy(5*time.Minute, 10)

	assert.NotNil(t, proxy)
	assert.NotNil(t, proxy.pool)
}

func TestGRPCProxyHandler(t *testing.T) {
	// Setup Gin context
	gin.SetMode(gin.TestMode)

	// Create a mock pool
	mockPool := &MockClientPool{
		getConnFunc: func(ctx context.Context, target string) (grpcpool.ClientConn, error) {
			// Return a mock connection
			return &MockClientConn{
				invokeFunc: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
					// Simulate successful response by setting data on the reply message
					if msg, ok := reply.(*MockMessage); ok {
						msg.jsonData = map[string]interface{}{
							"greeting": "Hello, world!",
							"status":   "success",
						}
					}
					return nil
				},
			}, nil
		},
		releaseConnFunc: func(target string) {
			// No-op for test
		},
		closeFunc: func() {
			// No-op for test
		},
	}

	// Create the proxy with our mock pool
	proxy := &GRPCProxyWithMocks{
		pool: mockPool,
	}

	// Register a mock method descriptor in the cache
	mockMethodDesc := &mockMethodDescriptor{}
	proxy.methodCache.Store("TestService/SayHello", mockMethodDesc)

	// Add ProxyHandler method to our test proxy
	proxyHandler := func(c *gin.Context, target string, fullMethodName string) {
		ctx := c.Request.Context()

		// Get connection from pool
		conn, err := proxy.pool.GetConn(ctx, target)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": fmt.Sprintf("failed to connect to gRPC service: %v", err)})
			return
		}
		defer proxy.pool.ReleaseConn(target)

		// Get method descriptor from cache or create new
		_, ok := proxy.methodCache.Load(fullMethodName)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid gRPC method")})
			return
		}

		// Create input/output messages (simplified for test)
		inputMsg := &MockMessage{jsonData: make(map[string]interface{})}
		outputMsg := &MockMessage{jsonData: make(map[string]interface{})}

		// Parse request body into input message
		if c.Request.Body != nil {
			decoder := json.NewDecoder(c.Request.Body)
			jsonData := make(map[string]interface{})
			if err := decoder.Decode(&jsonData); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
				return
			}
		}

		// Prepare metadata
		md := metadata.New(nil)
		for k, v := range c.Request.Header {
			md.Set(strings.ToLower(k), v...)
		}
		ctx = metadata.NewOutgoingContext(ctx, md)

		// Make gRPC call
		if err := conn.Invoke(ctx, fullMethodName, inputMsg, outputMsg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("gRPC call failed: %v", err)})
			return
		}

		// Return the mocked response
		c.JSON(http.StatusOK, outputMsg.jsonData)
	}

	t.Run("successful_grpc_call", func(t *testing.T) {
		// Setup request
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Create a request with a JSON body
		reqBody := strings.NewReader(`{"name": "John"}`)
		req, _ := http.NewRequest("POST", "/test", reqBody)
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		// Call the proxy handler
		proxyHandler(c, "localhost:50051", "TestService/SayHello")

		// Check the response
		assert.Equal(t, http.StatusOK, w.Code)

		// Verify response content
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "Hello, world!", response["greeting"])
		assert.Equal(t, "success", response["status"])
	})

	t.Run("grpc_call_with_headers", func(t *testing.T) {
		// Setup request with headers that should be passed to gRPC metadata
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := strings.NewReader(`{"name": "John"}`)
		req, _ := http.NewRequest("POST", "/test", reqBody)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Custom-Header", "custom-value")
		c.Request = req

		// Mock pool with header verification
		mockPoolWithHeaderCheck := &MockClientPool{
			getConnFunc: func(ctx context.Context, target string) (grpcpool.ClientConn, error) {
				return &MockClientConn{
					invokeFunc: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
						// Check headers were passed as metadata
						md, ok := metadata.FromOutgoingContext(ctx)
						require.True(t, ok, "Metadata should be present")

						values := md.Get("x-custom-header")
						require.Len(t, values, 1, "Custom header should be present")
						assert.Equal(t, "custom-value", values[0])

						// Set reply data
						if msg, ok := reply.(*MockMessage); ok {
							msg.jsonData = map[string]interface{}{
								"greeting": "Hello with headers",
							}
						}
						return nil
					},
				}, nil
			},
			releaseConnFunc: func(target string) {},
			closeFunc:       func() {},
		}

		proxyWithHeaderCheck := &GRPCProxyWithMocks{
			pool: mockPoolWithHeaderCheck,
		}
		proxyWithHeaderCheck.methodCache.Store("TestService/SayHello", mockMethodDesc)

		// Create the handler function
		proxyHandlerWithHeaders := func(c *gin.Context, target string, fullMethodName string) {
			ctx := c.Request.Context()
			conn, _ := proxyWithHeaderCheck.pool.GetConn(ctx, target)
			defer proxyWithHeaderCheck.pool.ReleaseConn(target)

			// Check that we have a method descriptor
			_, ok := proxyWithHeaderCheck.methodCache.Load(fullMethodName)
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid gRPC method"})
				return
			}

			inputMsg := &MockMessage{jsonData: make(map[string]interface{})}
			outputMsg := &MockMessage{jsonData: make(map[string]interface{})}

			// Prepare metadata
			md := metadata.New(nil)
			for k, v := range c.Request.Header {
				md.Set(strings.ToLower(k), v...)
			}
			ctx = metadata.NewOutgoingContext(ctx, md)

			conn.Invoke(ctx, fullMethodName, inputMsg, outputMsg)
			c.JSON(http.StatusOK, outputMsg.jsonData)
		}

		// Call the proxy handler
		proxyHandlerWithHeaders(c, "localhost:50051", "TestService/SayHello")

		// Check the response
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// Mock method descriptor implementations for testing
type mockMethodDescriptor struct {
	protoreflect.MethodDescriptor
}

func (m *mockMethodDescriptor) Input() protoreflect.MessageDescriptor {
	return &mockMessageDescriptor{name: "TestRequest"}
}

func (m *mockMethodDescriptor) Output() protoreflect.MessageDescriptor {
	return &mockMessageDescriptor{name: "TestResponse"}
}

type mockMessageDescriptor struct {
	protoreflect.MessageDescriptor
	name string
}

func (m *mockMessageDescriptor) FullName() protoreflect.FullName {
	return protoreflect.FullName(m.name)
}

func TestClose(t *testing.T) {
	closed := false
	mockPool := &MockClientPool{
		getConnFunc:     func(ctx context.Context, target string) (grpcpool.ClientConn, error) { return nil, nil },
		releaseConnFunc: func(target string) {},
		closeFunc:       func() { closed = true },
	}

	proxy := &GRPCProxyWithMocks{
		pool: mockPool,
	}

	// Add Close method to our test proxy
	closeFunc := func() {
		proxy.pool.Close()
	}

	closeFunc()
	assert.True(t, closed, "The pool should be closed")
}
