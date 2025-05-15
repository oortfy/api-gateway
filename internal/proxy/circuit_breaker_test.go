package proxy

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Define a test handler that can succeed or fail based on test conditions
type testHandler struct {
	shouldSucceed bool
	failureCount  int
	successCount  int
	mutex         sync.Mutex
}

func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.shouldSucceed {
		h.successCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	} else {
		h.failureCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Failure"))
	}
}

func (h *testHandler) reset() {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.failureCount = 0
	h.successCount = 0
}

func (h *testHandler) getSuccessCount() int {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	return h.successCount
}

func (h *testHandler) getFailureCount() int {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	return h.failureCount
}

func TestNewCircuitBreaker(t *testing.T) {
	log := &mockLogger{}
	config := CircuitBreakerConfig{
		Threshold:     5,
		Timeout:       10 * time.Second,
		MaxConcurrent: 100,
	}

	cb := NewCircuitBreaker("test", config, log)

	assert.NotNil(t, cb)
	assert.Equal(t, "test", cb.name)
	assert.Equal(t, config, cb.config)
	assert.Equal(t, Closed, cb.state)
	assert.Equal(t, 0, cb.failures)
	assert.Equal(t, 0, cb.totalRequests)
	assert.NotNil(t, cb.mutex)
}

func TestCircuitBreakerTripping(t *testing.T) {
	log := &mockLogger{}
	config := CircuitBreakerConfig{
		Threshold:     3,               // Trip after 3 failures
		Timeout:       2 * time.Second, // Reset after 2 seconds
		MaxConcurrent: 100,
	}

	cb := NewCircuitBreaker("test", config, log)
	require.NotNil(t, cb)

	// Initially the circuit should be closed
	assert.Equal(t, Closed, cb.state)

	// Record 3 failures to trip the circuit
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	// Circuit should now be open
	assert.Equal(t, Open, cb.state)

	// Reset counter and manually close the circuit
	cb.mutex.Lock()
	cb.failures = 0
	cb.state = Closed
	cb.mutex.Unlock()

	// Circuit should be closed again
	assert.Equal(t, Closed, cb.state)
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	log := &mockLogger{}
	config := CircuitBreakerConfig{
		Threshold:     3,
		Timeout:       1 * time.Second, // 1 second timeout for easier testing
		MaxConcurrent: 100,
	}

	cb := NewCircuitBreaker("test", config, log)

	// Trip the circuit
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	assert.Equal(t, Open, cb.state)

	// Wait for the timeout to transition to half-open
	time.Sleep(1100 * time.Millisecond) // Slightly longer than the timeout

	// Try to get a request through, which should set it to half-open
	assert.True(t, cb.AllowRequest())

	// Should be half-open now
	assert.Equal(t, HalfOpen, cb.state)

	// Record a success to close the circuit
	cb.RecordSuccess()
	assert.Equal(t, Closed, cb.state)
}

func TestCircuitBreakerWrapper(t *testing.T) {
	log := &mockLogger{}
	config := CircuitBreakerConfig{
		Threshold:     3,
		Timeout:       1 * time.Second,
		MaxConcurrent: 100,
	}

	cb := NewCircuitBreaker("test", config, log)

	// Create a test handler that fails initially
	handler := &testHandler{shouldSucceed: false}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Call Execute directly
		err := cb.Execute(r, handler, w)
		if err != nil {
			// Circuit already handled the response
			return
		}
	}))
	defer server.Close()

	// Send requests until the circuit opens
	client := &http.Client{Timeout: 100 * time.Millisecond}
	for i := 0; i < 5; i++ {
		resp, _ := client.Get(server.URL)
		if resp != nil {
			resp.Body.Close()
		}
	}

	// Circuit should be open now
	assert.Equal(t, Open, cb.state)

	// Send another request which should be short-circuited
	beforeFailures := handler.getFailureCount()
	resp, _ := client.Get(server.URL)
	if resp != nil {
		resp.Body.Close()
	}

	// The handler should not have been called again
	assert.Equal(t, beforeFailures, handler.getFailureCount())

	// Wait for the circuit to transition to half-open
	time.Sleep(1100 * time.Millisecond)

	// Now make the handler succeed
	handler.shouldSucceed = true

	// Send a request
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	resp.Body.Close()

	// Circuit should be closed after success
	assert.Equal(t, Closed, cb.state)
}
