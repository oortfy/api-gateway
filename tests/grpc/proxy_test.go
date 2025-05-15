package grpc_test

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/types/known/emptypb"

	"api-gateway/internal/proxy"
)

func setupTestServer(t *testing.T) (string, func()) {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	healthServer := &mockHealthServer{healthy: true}
	grpc_health_v1.RegisterHealthServer(server, healthServer)

	go server.Serve(lis)

	cleanup := func() {
		server.Stop()
		lis.Close()
	}

	return lis.Addr().String(), cleanup
}

func TestGRPCProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	serverAddr, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a new proxy instance
	p := proxy.NewGRPCProxy(time.Minute, 2)
	defer p.Close()

	t.Run("ProxyHandler handles valid request", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Create a test request with empty body (health check request is empty)
		c.Request = httptest.NewRequest("POST", "/test", bytes.NewBuffer([]byte("{}")))
		c.Request.Header.Set("Content-Type", "application/json")

		// Call proxy handler with mock server address
		p.ProxyHandler(c, serverAddr, "grpc.health.v1.Health/Check")

		// Should succeed since we have a mock health server
		assert.Equal(t, http.StatusOK, w.Code)
		if w.Code != http.StatusOK {
			t.Logf("Response body: %s", w.Body.String())
		}
	})

	t.Run("ProxyHandler handles invalid method", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/test", nil)

		// Call with invalid method name
		p.ProxyHandler(c, serverAddr, "invalid/method/name")

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ProxyHandler handles invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Create invalid JSON request
		c.Request = httptest.NewRequest("POST", "/test", bytes.NewBufferString("{invalid json}"))
		c.Request.Header.Set("Content-Type", "application/json")

		p.ProxyHandler(c, serverAddr, "grpc.health.v1.Health/Check")

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestServer represents the gRPC server API
type TestServer interface {
	TestMethod(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
}

// mockService implements TestServer
type mockService struct{}

func (s *mockService) TestMethod(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func setupMockGRPCServer(t *testing.T) (*grpc.Server, string) {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	s := grpc.NewServer()
	// Register your mock service here
	go s.Serve(lis)

	return s, lis.Addr().String()
}
