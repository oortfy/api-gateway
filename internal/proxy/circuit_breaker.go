package proxy

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"api-gateway/pkg/logger"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	// Closed means the circuit breaker is closed (allowing traffic)
	Closed CircuitBreakerState = iota
	// Open means the circuit breaker is open (blocking traffic)
	Open
	// HalfOpen means the circuit breaker is allowing a test request
	HalfOpen
)

// CircuitBreakerConfig contains configuration for a circuit breaker
type CircuitBreakerConfig struct {
	// Threshold is the number of consecutive failures before opening the circuit
	Threshold int
	// Timeout is the duration to wait before transitioning from Open to HalfOpen
	Timeout time.Duration
	// MaxConcurrent is the maximum number of concurrent requests (optional)
	MaxConcurrent int
}

// DefaultCircuitBreakerConfig returns a default circuit breaker configuration
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Threshold:     5,
		Timeout:       30 * time.Second,
		MaxConcurrent: 100,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name          string
	state         CircuitBreakerState
	config        CircuitBreakerConfig
	failures      int
	lastFailure   time.Time
	mutex         sync.RWMutex
	inFlight      int
	inFlightMutex sync.Mutex
	log           logger.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, config CircuitBreakerConfig, log logger.Logger) *CircuitBreaker {
	if config.Threshold <= 0 {
		config.Threshold = 5
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}

	return &CircuitBreaker{
		name:        name,
		state:       Closed,
		config:      config,
		failures:    0,
		lastFailure: time.Time{},
		log:         log,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(req *http.Request, next http.Handler, w http.ResponseWriter) error {
	// Check if the circuit is open
	if !cb.AllowRequest() {
		cb.log.Debug("Circuit breaker open, request rejected",
			logger.String("circuit", cb.name),
			logger.String("path", req.URL.Path),
		)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return errors.New("circuit open")
	}

	// Increment in-flight requests
	if !cb.acquireSemaphore() {
		cb.log.Debug("Circuit breaker max concurrent requests exceeded",
			logger.String("circuit", cb.name),
			logger.String("path", req.URL.Path),
		)
		http.Error(w, "Too many requests", http.StatusTooManyRequests)
		return errors.New("max concurrent requests")
	}

	// Decrement in-flight requests when done
	defer cb.releaseSemaphore()

	// Create a custom response writer to capture status code
	crw := &customResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	// Process the request
	next.ServeHTTP(crw, req)

	// If status code indicates a server error, record a failure
	if crw.statusCode >= 500 {
		cb.RecordFailure()
	} else {
		cb.RecordSuccess()
	}

	return nil
}

// AllowRequest checks if a request should be allowed based on circuit state
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mutex.RLock()
	state := cb.state
	cb.mutex.RUnlock()

	switch state {
	case Closed:
		// Circuit is closed, requests are allowed
		return true
	case Open:
		// Check if timeout has elapsed to try a test request
		cb.mutex.Lock()
		defer cb.mutex.Unlock()

		// If timeout has elapsed, transition to half-open
		if time.Since(cb.lastFailure) > cb.config.Timeout {
			cb.state = HalfOpen
			cb.log.Info("Circuit breaker transitioned to half-open",
				logger.String("circuit", cb.name),
			)
			return true
		}

		return false
	case HalfOpen:
		// In half-open state, allow only one request for testing
		cb.mutex.RLock()
		defer cb.mutex.RUnlock()
		return true
	default:
		return true
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	switch cb.state {
	case HalfOpen:
		// If successful in half-open state, close the circuit
		cb.failures = 0
		cb.state = Closed
		cb.log.Info("Circuit breaker closed after successful test request",
			logger.String("circuit", cb.name),
		)
	case Closed:
		// Reset failure count in closed state
		cb.failures = 0
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.lastFailure = time.Now()

	switch cb.state {
	case HalfOpen:
		// If failed in half-open state, open the circuit again
		cb.state = Open
		cb.log.Warn("Circuit breaker reopened after failed test request",
			logger.String("circuit", cb.name),
		)
	case Closed:
		// Increment failures in closed state
		cb.failures++

		// If failures exceed threshold, open the circuit
		if cb.failures >= cb.config.Threshold {
			cb.state = Open
			cb.log.Warn("Circuit breaker opened after consecutive failures",
				logger.String("circuit", cb.name),
				logger.Int("failures", cb.failures),
			)
		}
	}
}

// acquireSemaphore attempts to acquire a semaphore for concurrent request limiting
func (cb *CircuitBreaker) acquireSemaphore() bool {
	if cb.config.MaxConcurrent <= 0 {
		return true
	}

	cb.inFlightMutex.Lock()
	defer cb.inFlightMutex.Unlock()

	if cb.inFlight >= cb.config.MaxConcurrent {
		return false
	}

	cb.inFlight++
	return true
}

// releaseSemaphore releases a semaphore
func (cb *CircuitBreaker) releaseSemaphore() {
	if cb.config.MaxConcurrent <= 0 {
		return
	}

	cb.inFlightMutex.Lock()
	defer cb.inFlightMutex.Unlock()

	if cb.inFlight > 0 {
		cb.inFlight--
	}
}

// customResponseWriter is a wrapper around http.ResponseWriter that captures the status code
type customResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (crw *customResponseWriter) WriteHeader(statusCode int) {
	crw.statusCode = statusCode
	crw.ResponseWriter.WriteHeader(statusCode)
}
