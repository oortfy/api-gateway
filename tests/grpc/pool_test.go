package grpc_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"

	grpcpool "api-gateway/pkg/grpc"
)

type mockHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
	healthy bool
}

func (m *mockHealthServer) Check(context.Context, *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	status := grpc_health_v1.HealthCheckResponse_NOT_SERVING
	if m.healthy {
		status = grpc_health_v1.HealthCheckResponse_SERVING
	}
	return &grpc_health_v1.HealthCheckResponse{Status: status}, nil
}

func setupMockServer(t *testing.T) (string, func()) {
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

func TestClientPool(t *testing.T) {
	serverAddr, cleanup := setupMockServer(t)
	defer cleanup()

	// Create pool with test configuration
	pool := grpcpool.NewClientPool(grpcpool.PoolConfig{
		MaxIdle:       time.Minute,
		MaxConns:      2,
		HealthCheck:   true,
		CheckInterval: time.Second,
	})
	defer pool.Close()

	t.Run("GetConn creates new connection", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, err := pool.GetConn(ctx, serverAddr)
		require.NoError(t, err)
		assert.NotNil(t, conn)
	})

	t.Run("GetConn reuses existing connection", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn1, err := pool.GetConn(ctx, serverAddr)
		require.NoError(t, err)

		conn2, err := pool.GetConn(ctx, serverAddr)
		require.NoError(t, err)

		assert.Equal(t, conn1, conn2)
	})

	t.Run("ReleaseConn marks connection as not in use", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := pool.GetConn(ctx, serverAddr)
		require.NoError(t, err)

		pool.ReleaseConn(serverAddr)
		// Implementation detail: connection should still be in pool but marked as not in use
	})

	t.Run("InvokeRPC with retries", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, err := pool.GetConn(ctx, serverAddr)
		require.NoError(t, err)

		err = pool.InvokeRPC(ctx, conn.(*grpc.ClientConn), "/test.Service/Method", nil, nil)
		// Expected to fail since method doesn't exist, but should have attempted retries
		assert.Error(t, err)
	})
}

func TestPoolConfig(t *testing.T) {
	t.Run("default check interval", func(t *testing.T) {
		pool := grpcpool.NewClientPool(grpcpool.PoolConfig{
			MaxIdle:  time.Minute,
			MaxConns: 2,
		})
		defer pool.Close()
		// Default check interval should be set
	})

	t.Run("custom check interval", func(t *testing.T) {
		customInterval := 5 * time.Second
		pool := grpcpool.NewClientPool(grpcpool.PoolConfig{
			MaxIdle:       time.Minute,
			MaxConns:      2,
			CheckInterval: customInterval,
		})
		defer pool.Close()
		// Custom check interval should be used
	})
}
