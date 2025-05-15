package etcd_discovery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Create a mock implementation for testing without requiring etcd server
type mockServiceDiscovery struct {
	services map[string][]string
}

func newMockServiceDiscovery() *mockServiceDiscovery {
	return &mockServiceDiscovery{
		services: make(map[string][]string),
	}
}

func (m *mockServiceDiscovery) RegisterService(prefix, serviceName, serviceAddr string, ttl int64) error {
	key := prefix + "/" + serviceName
	if _, exists := m.services[key]; !exists {
		m.services[key] = []string{}
	}
	m.services[key] = append(m.services[key], serviceAddr)
	return nil
}

func (m *mockServiceDiscovery) DiscoverServices(prefix, serviceName string) ([]string, error) {
	key := prefix + "/" + serviceName
	return m.services[key], nil
}

// Test registration with mocks
func TestServiceRegistration(t *testing.T) {
	mock := newMockServiceDiscovery()

	// Register a service
	prefix := "services"
	serviceName := "test-service"
	serviceAddr := "127.0.0.1:8888"

	err := mock.RegisterService(prefix, serviceName, serviceAddr, 10)
	require.NoError(t, err)

	// Check registration was successful
	services, err := mock.DiscoverServices(prefix, serviceName)
	require.NoError(t, err)
	assert.Contains(t, services, serviceAddr)
}

// Test the service discovery response handling
func TestExtractAddrs(t *testing.T) {
	sd := &ServiceDiscovery{
		services: make(map[string][]string),
	}

	// Mock a clientv3.GetResponse
	response := &clientv3.GetResponse{
		Kvs: []*mvccpb.KeyValue{
			{
				Key: []byte("/services/api/10.0.0.1:8080"),
			},
			{
				Key: []byte("/services/api/10.0.0.2:8080"),
			},
		},
	}

	addrs := sd.extractAddrs(response)
	assert.Len(t, addrs, 2)
	assert.Contains(t, addrs, "10.0.0.1:8080")
	assert.Contains(t, addrs, "10.0.0.2:8080")
}

// Skip tests that require a real etcd server
func TestWithRealEtcd(t *testing.T) {
	t.Skip("Skipping tests that require a real etcd server")

	// Registration
	t.Run("registration", func(t *testing.T) {
		var endpoints = []string{"127.0.0.1:2379"}
		sd, err := NewServiceDiscovery(endpoints, 5*time.Second)
		if err != nil {
			t.Logf("Connect etcd error: %v", err)
			t.Skip("Skipping test due to etcd connection error")
			return
		}
		defer sd.Close()

		// Register service
		prefix := "services"
		serviceName := "test-service"
		serviceAddr := "127.0.0.1:8888"
		err = sd.RegisterService(prefix, serviceName, serviceAddr, 10)
		require.NoError(t, err)
	})

	// Discovery
	t.Run("discovery", func(t *testing.T) {
		var endpoints = []string{"127.0.0.1:2379"}
		sd, err := NewServiceDiscovery(endpoints, 5*time.Second)
		if err != nil {
			t.Logf("Connect etcd error: %v", err)
			t.Skip("Skipping test due to etcd connection error")
			return
		}
		defer sd.Close()

		// Discover services
		prefix := "services"
		serviceName := "test-service"
		addrs, err := sd.DiscoverServices(prefix, serviceName)
		require.NoError(t, err)
		t.Logf("Discovered services: %v", addrs)
	})
}
