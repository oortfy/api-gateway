package grpc

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var (
	// Metrics
	activeConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "grpc_pool_active_connections",
		Help: "The current number of active gRPC connections",
	})

	connectionErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "grpc_pool_connection_errors_total",
		Help: "The total number of gRPC connection errors",
	})

	rpcDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_rpc_duration_seconds",
			Help:    "The duration of gRPC RPCs in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	rpcErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_rpc_errors_total",
			Help: "The total number of gRPC RPC errors",
		},
		[]string{"method"},
	)

	healthCheckStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "grpc_health_check_status",
			Help: "The health status of gRPC connections (1 for healthy, 0 for unhealthy)",
		},
		[]string{"target"},
	)
)

// ClientConn represents a gRPC client connection interface
type ClientConn interface {
	Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error
}

type ClientPool struct {
	mu            sync.RWMutex
	clients       map[string]*pooledClient
	maxIdle       time.Duration
	maxConns      int
	healthCheck   bool
	checkInterval time.Duration
}

type pooledClient struct {
	conn       *grpc.ClientConn
	lastUsed   time.Time
	inUse      bool
	targetAddr string
	healthy    bool
}

type PoolConfig struct {
	MaxIdle       time.Duration
	MaxConns      int
	HealthCheck   bool
	CheckInterval time.Duration
}

// RetryConfig defines the retry behavior
type RetryConfig struct {
	MaxAttempts       int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
}

// DefaultRetryConfig provides sensible default retry settings
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:       3,
	InitialBackoff:    100 * time.Millisecond,
	MaxBackoff:        2 * time.Second,
	BackoffMultiplier: 2.0,
}

func NewClientPool(config PoolConfig) *ClientPool {
	if config.CheckInterval == 0 {
		config.CheckInterval = time.Second * 30
	}

	pool := &ClientPool{
		clients:       make(map[string]*pooledClient),
		maxIdle:       config.MaxIdle,
		maxConns:      config.MaxConns,
		healthCheck:   config.HealthCheck,
		checkInterval: config.CheckInterval,
	}

	// Start cleanup goroutine
	go pool.cleanup()

	// Start health check goroutine if enabled
	if config.HealthCheck {
		go pool.healthCheckLoop()
	}

	return pool
}

func (p *ClientPool) GetConn(ctx context.Context, target string) (ClientConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check for existing connection
	if client, exists := p.clients[target]; exists {
		if client.healthy && client.conn.GetState() == connectivity.Ready {
			client.lastUsed = time.Now()
			client.inUse = true
			return client.conn, nil
		}
		// Connection not healthy or ready, remove it
		delete(p.clients, target)
		activeConnections.Dec()
	}

	// Create new connection
	conn, err := grpc.DialContext(ctx, target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(16*1024*1024)), // 16MB
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(16*1024*1024)), // 16MB
		grpc.WithBlock(),
	)
	if err != nil {
		connectionErrors.Inc()
		return nil, err
	}

	client := &pooledClient{
		conn:       conn,
		lastUsed:   time.Now(),
		inUse:      true,
		targetAddr: target,
		healthy:    true, // Assume healthy initially
	}
	p.clients[target] = client
	activeConnections.Inc()

	// Perform initial health check if enabled
	if p.healthCheck {
		client.healthy = p.checkHealth(ctx, client)
		if client.healthy {
			healthCheckStatus.WithLabelValues(target).Set(1)
		} else {
			healthCheckStatus.WithLabelValues(target).Set(0)
		}
	}

	return conn, nil
}

func (p *ClientPool) ReleaseConn(target string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if client, exists := p.clients[target]; exists {
		client.inUse = false
		client.lastUsed = time.Now()
	}
}

func (p *ClientPool) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		now := time.Now()
		for target, client := range p.clients {
			if !client.inUse && now.Sub(client.lastUsed) > p.maxIdle {
				client.conn.Close()
				delete(p.clients, target)
				activeConnections.Dec()
				healthCheckStatus.DeleteLabelValues(target)
			}
		}
		p.mu.Unlock()
	}
}

func (p *ClientPool) healthCheckLoop() {
	ticker := time.NewTicker(p.checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.RLock()
		clients := make([]*pooledClient, 0, len(p.clients))
		for _, client := range p.clients {
			clients = append(clients, client)
		}
		p.mu.RUnlock()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		for _, client := range clients {
			client.healthy = p.checkHealth(ctx, client)
		}
		cancel()
	}
}

func (p *ClientPool) checkHealth(ctx context.Context, client *pooledClient) bool {
	healthClient := grpc_health_v1.NewHealthClient(client.conn)
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	healthy := err == nil && resp.Status == grpc_health_v1.HealthCheckResponse_SERVING

	if healthy {
		healthCheckStatus.WithLabelValues(client.targetAddr).Set(1)
	} else {
		healthCheckStatus.WithLabelValues(client.targetAddr).Set(0)
	}

	return healthy
}

func (p *ClientPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, client := range p.clients {
		client.conn.Close()
	}
	p.clients = make(map[string]*pooledClient)
}

// InvokeRPC makes a gRPC call using the provided connection with retry logic
func (p *ClientPool) InvokeRPC(ctx context.Context, conn *grpc.ClientConn, method string, args interface{}, reply interface{}) error {
	var lastErr error
	attempt := 0
	backoff := DefaultRetryConfig.InitialBackoff

	timer := prometheus.NewTimer(rpcDuration.WithLabelValues(method))
	defer timer.ObserveDuration()

	for attempt < DefaultRetryConfig.MaxAttempts {
		select {
		case <-ctx.Done():
			rpcErrors.WithLabelValues(method).Inc()
			return ctx.Err()
		default:
			err := conn.Invoke(ctx, method, args, reply)
			if err == nil {
				return nil
			}

			lastErr = err
			attempt++
			rpcErrors.WithLabelValues(method).Inc()

			// Don't retry if this is the last attempt
			if attempt >= DefaultRetryConfig.MaxAttempts {
				break
			}

			// Calculate next backoff duration
			backoff = time.Duration(float64(backoff) * DefaultRetryConfig.BackoffMultiplier)
			if backoff > DefaultRetryConfig.MaxBackoff {
				backoff = DefaultRetryConfig.MaxBackoff
			}

			// Wait before retrying
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}
	}

	return lastErr
}
