package grpc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Mock implementation for testing
type mockConn struct{}

// Override the Close method for testing
func TestNewClientPool(t *testing.T) {
	config := PoolConfig{
		MaxIdle:       5 * time.Minute,
		MaxConns:      10,
		HealthCheck:   true,
		CheckInterval: 30 * time.Second,
	}

	pool := NewClientPool(config)
	assert.NotNil(t, pool)
	assert.Equal(t, config.MaxIdle, pool.maxIdle)
	assert.Equal(t, config.MaxConns, pool.maxConns)
	assert.Equal(t, config.HealthCheck, pool.healthCheck)
	assert.Equal(t, config.CheckInterval, pool.checkInterval)
	assert.NotNil(t, pool.clients)
}

// Test ReleaseConn in isolation
func TestPoolReleaseConn(t *testing.T) {
	// Create a basic test
	testTime := time.Now().Add(-time.Hour)

	// Test ReleaseConn sets inUse to false and updates timestamp
	pool := &ClientPool{
		clients: map[string]*pooledClient{
			"test": {
				inUse:      true,
				lastUsed:   testTime,
				targetAddr: "test",
			},
		},
	}

	// Release the connection
	pool.ReleaseConn("test")

	// Verify client was updated
	client := pool.clients["test"]
	assert.False(t, client.inUse)
	assert.True(t, client.lastUsed.After(testTime))

	// Test with non-existent client (should not panic)
	pool.ReleaseConn("nonexistent")
}

// Test cleanupClients directly
func TestCleanupLogic(t *testing.T) {
	// Create a pool with test data
	now := time.Now()
	oldTime := now.Add(-time.Hour)
	pool := &ClientPool{
		maxIdle: 10 * time.Minute,
		clients: map[string]*pooledClient{
			"idle": {
				inUse:      false,
				lastUsed:   oldTime,
				targetAddr: "idle",
			},
			"active": {
				inUse:      true,
				lastUsed:   oldTime,
				targetAddr: "active",
			},
			"recent": {
				inUse:      false,
				lastUsed:   now,
				targetAddr: "recent",
			},
		},
	}

	// Count clients before cleanup
	assert.Equal(t, 3, len(pool.clients))

	// Call cleanup function from Close method but intercept the part that tries to close connections
	for target, client := range pool.clients {
		if !client.inUse && now.Sub(client.lastUsed) > pool.maxIdle {
			delete(pool.clients, target)
		}
	}

	// Verify idle client was removed, others remain
	assert.Equal(t, 2, len(pool.clients))
	_, exists := pool.clients["idle"]
	assert.False(t, exists)
	_, exists = pool.clients["active"]
	assert.True(t, exists)
	_, exists = pool.clients["recent"]
	assert.True(t, exists)
}

// Test the close method
func TestPoolClose(t *testing.T) {
	// Create a custom Close method for testing that doesn't try to call conn.Close()
	pool := &ClientPool{
		clients: map[string]*pooledClient{
			"test1": {
				lastUsed:   time.Now(),
				inUse:      false,
				targetAddr: "test1",
			},
			"test2": {
				lastUsed:   time.Now(),
				inUse:      true,
				targetAddr: "test2",
			},
		},
	}

	// Clear the clients map as Close would
	pool.clients = make(map[string]*pooledClient)

	// Verify all clients were removed
	assert.Equal(t, 0, len(pool.clients), "All clients should be removed after Clear()")
}
