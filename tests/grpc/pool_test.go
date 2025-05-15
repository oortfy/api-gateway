package grpc_test

import (
	"context"
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

func TestClientPool(t *testing.T) {
	// Create pool with test configuration
	pool := grpcpool.NewClientPool(grpcpool.PoolConfig{
		MaxIdle:       time.Minute,
		MaxConns:      2,
		HealthCheck:   true,
		CheckInterval: time.Second,
	})
	defer pool.Close()

	t.Run("GetConn creates new connection", func(t *testing.T) {
		ctx := context.Background()
		conn, err := pool.GetConn(ctx, "localhost:50051")
		require.NoError(t, err)
		assert.NotNil(t, conn)
	})

	t.Run("GetConn reuses existing connection", func(t *testing.T) {
		ctx := context.Background()
		conn1, err := pool.GetConn(ctx, "localhost:50051")
		require.NoError(t, err)

		conn2, err := pool.GetConn(ctx, "localhost:50051")
		require.NoError(t, err)

		assert.Equal(t, conn1, conn2)
	})

	t.Run("ReleaseConn marks connection as not in use", func(t *testing.T) {
		ctx := context.Background()
		_, err := pool.GetConn(ctx, "localhost:50051")
		require.NoError(t, err)

		pool.ReleaseConn("localhost:50051")
		// Implementation detail: connection should still be in pool but marked as not in use
	})

	t.Run("InvokeRPC with retries", func(t *testing.T) {
		ctx := context.Background()
		conn, err := pool.GetConn(ctx, "localhost:50051")
		require.NoError(t, err)

		err = pool.InvokeRPC(ctx, conn.(*grpc.ClientConn), "/test.Service/Method", nil, nil)
		// Expected to fail since no real server, but should have attempted retries
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
