package grpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"api-gateway/internal/proxy"
)

func TestGRPCProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a new proxy instance
	p := proxy.NewGRPCProxy(time.Minute, 2)
	defer p.Close()

	t.Run("ProxyHandler handles valid request", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Create a test request with JSON body
		body := map[string]interface{}{
			"name": "test",
		}
		jsonBody, _ := json.Marshal(body)
		c.Request = httptest.NewRequest("POST", "/test", bytes.NewBuffer(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		// Call proxy handler
		p.ProxyHandler(c, "localhost:50051", "test.Service/Method")

		// Since we don't have a real gRPC server, we expect an error response
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("ProxyHandler handles invalid method", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/test", nil)

		// Call with invalid method name
		p.ProxyHandler(c, "localhost:50051", "invalid/method/name")

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ProxyHandler handles invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Create invalid JSON request
		c.Request = httptest.NewRequest("POST", "/test", bytes.NewBufferString("{invalid json}"))
		c.Request.Header.Set("Content-Type", "application/json")

		p.ProxyHandler(c, "localhost:50051", "test.Service/Method")

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
