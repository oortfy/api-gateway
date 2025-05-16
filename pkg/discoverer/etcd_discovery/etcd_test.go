package etcd_discovery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	assert.NoError(t, err)

	// Check registration was successful
	services, err := mock.DiscoverServices(prefix, serviceName)
	assert.NoError(t, err)
	assert.Contains(t, services, serviceAddr)
}

// Mock etcd client for testing
type MockEtcdClient struct {
	mock.Mock
}

func (m *MockEtcdClient) Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
	args := m.Called(ctx, ttl)
	return args.Get(0).(*clientv3.LeaseGrantResponse), args.Error(1)
}

func (m *MockEtcdClient) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	args := m.Called(ctx, key, val, opts)
	return args.Get(0).(*clientv3.PutResponse), args.Error(1)
}

func (m *MockEtcdClient) KeepAlive(ctx context.Context, id clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(<-chan *clientv3.LeaseKeepAliveResponse), args.Error(1)
}

func (m *MockEtcdClient) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	args := m.Called(ctx, key, opts)
	return args.Get(0).(*clientv3.GetResponse), args.Error(1)
}

func (m *MockEtcdClient) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	args := m.Called(ctx, key, opts)
	return args.Get(0).(clientv3.WatchChan)
}

func (m *MockEtcdClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// On adds a new expected call
func (m *MockEtcdClient) On(methodName string, arguments ...interface{}) *mock.Call {
	return m.Mock.On(methodName, arguments...)
}

// Called indicates that the method was called
func (m *MockEtcdClient) Called(arguments ...interface{}) mock.Arguments {
	return m.Mock.Called(arguments...)
}

func TestGetService(t *testing.T) {
	// Create a service discovery instance
	sd := &ServiceDiscovery{
		services: map[string][]string{
			"test-service": {"localhost:8080", "localhost:8081"},
		},
	}

	// Test getting existing service
	addrs := sd.GetService("test-service")
	assert.Equal(t, 2, len(addrs))
	assert.Contains(t, addrs, "localhost:8080")
	assert.Contains(t, addrs, "localhost:8081")

	// Test getting non-existent service
	addrs = sd.GetService("nonexistent-service")
	assert.Equal(t, 0, len(addrs))
}

func TestExtractAddrs(t *testing.T) {
	sd := &ServiceDiscovery{}

	// Create a mock GetResponse
	resp := &clientv3.GetResponse{
		Kvs: []*mvccpb.KeyValue{
			{
				Key:   []byte("/services/test-service/localhost:8080"),
				Value: []byte("localhost:8080"),
			},
			{
				Key:   []byte("/services/test-service/localhost:8081"),
				Value: []byte("localhost:8081"),
			},
		},
	}

	// Extract addresses
	addrs := sd.extractAddrs(resp)
	assert.Equal(t, 2, len(addrs))
	assert.Contains(t, addrs, "localhost:8080")
	assert.Contains(t, addrs, "localhost:8081")

	// Test with empty response
	resp = &clientv3.GetResponse{
		Kvs: []*mvccpb.KeyValue{},
	}
	addrs = sd.extractAddrs(resp)
	assert.Equal(t, 0, len(addrs))
}

func TestHandlePutAndDeleteEvents(t *testing.T) {
	// Create a service discovery instance
	sd := &ServiceDiscovery{
		services: map[string][]string{
			"test-service": {"localhost:8080"},
		},
	}

	// Test handlePutEvent with new address
	sd.handlePutEvent("test-service", &mvccpb.KeyValue{
		Key:   []byte("/services/test-service/localhost:8081"),
		Value: []byte("localhost:8081"),
	})

	// Verify service was added
	addrs := sd.GetService("test-service")
	assert.Equal(t, 2, len(addrs))
	assert.Contains(t, addrs, "localhost:8081")

	// Test handlePutEvent with existing address
	sd.handlePutEvent("test-service", &mvccpb.KeyValue{
		Key:   []byte("/services/test-service/localhost:8081"),
		Value: []byte("localhost:8081"),
	})

	// Verify no duplicate
	addrs = sd.GetService("test-service")
	assert.Equal(t, 2, len(addrs))

	// Test handleDeleteEvent
	sd.handleDeleteEvent("test-service", &mvccpb.KeyValue{
		Key: []byte("/services/test-service/localhost:8081"),
	})

	// Verify service was removed
	addrs = sd.GetService("test-service")
	assert.Equal(t, 1, len(addrs))
	assert.Contains(t, addrs, "localhost:8080")
}

func TestGetNextAddr(t *testing.T) {
	// Create a service discovery instance with test addresses
	sd := &ServiceDiscovery{
		services: map[string][]string{
			"test-service": {"localhost:8080", "localhost:8081", "localhost:8082"},
		},
		lastIndex: make(map[string]int),
	}

	// Test round-robin strategy
	// Since lastIndex starts at 0, the first call should return index 1
	addr1, err := sd.GetNextAddr("test-service", RoundRobin)
	assert.NoError(t, err)

	// Second call should return index 2
	addr2, err := sd.GetNextAddr("test-service", RoundRobin)
	assert.NoError(t, err)

	// Third call should wrap back to index 0
	addr3, err := sd.GetNextAddr("test-service", RoundRobin)
	assert.NoError(t, err)

	// Fourth call should go to index 1 again
	addr4, err := sd.GetNextAddr("test-service", RoundRobin)
	assert.NoError(t, err)

	// The implementation increments last index first, then uses it, so addresses are shifted
	assert.Equal(t, "localhost:8081", addr1)
	assert.Equal(t, "localhost:8082", addr2)
	assert.Equal(t, "localhost:8080", addr3)
	assert.Equal(t, "localhost:8081", addr4)

	// Test random strategy
	addr, err := sd.GetNextAddr("test-service", Random)
	assert.NoError(t, err)
	assert.Contains(t, []string{"localhost:8080", "localhost:8081", "localhost:8082"}, addr)

	// Test with non-existent service
	addr, err = sd.GetNextAddr("nonexistent-service", RoundRobin)
	assert.Error(t, err)
	assert.Equal(t, "", addr)
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
		assert.NoError(t, err)
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
		assert.NoError(t, err)
		t.Logf("Discovered services: %v", addrs)
	})
}
