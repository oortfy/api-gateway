package proxy

import (
	"errors"
	"fmt"
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

// String returns a string representation of the circuit breaker state
func (s CircuitBreakerState) String() string {
	switch s {
	case Closed:
		return "CLOSED"
	case Open:
		return "OPEN"
	case HalfOpen:
		return "HALF-OPEN"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(s))
	}
}

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
	totalRequests int
	totalFailures int
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, config CircuitBreakerConfig, log logger.Logger) *CircuitBreaker {
	if config.Threshold <= 0 {
		config.Threshold = 5
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}

	cb := &CircuitBreaker{
		name:        name,
		state:       Closed,
		config:      config,
		failures:    0,
		lastFailure: time.Time{},
		log:         log,
	}

	log.Info("Circuit breaker created",
		logger.String("name", name),
		logger.String("state", cb.state.String()),
		logger.Int("threshold", config.Threshold),
		logger.Int("timeout_seconds", int(config.Timeout.Seconds())),
		logger.Int("max_concurrent", config.MaxConcurrent))

	return cb
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(req *http.Request, next http.Handler, w http.ResponseWriter) error {
	// Check if the circuit is open
	cb.mutex.RLock()
	currentState := cb.state
	currentFailures := cb.failures
	cb.mutex.RUnlock()

	cb.log.Debug("Circuit breaker status check",
		logger.String("circuit", cb.name),
		logger.String("path", req.URL.Path),
		logger.String("state", currentState.String()),
		logger.Int("failures", currentFailures),
		logger.Int("threshold", cb.config.Threshold))

	if !cb.AllowRequest() {
		cb.log.Info("Circuit breaker open, request rejected",
			logger.String("circuit", cb.name),
			logger.String("path", req.URL.Path),
			logger.String("method", req.Method),
			logger.String("state", currentState.String()),
			logger.Int("failures", currentFailures),
			logger.Int("threshold", cb.config.Threshold))

		w.Header().Set("X-Circuit-Breaker", "open")
		http.Error(w, "Service temporarily unavailable (circuit breaker open)", http.StatusServiceUnavailable)
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
	if crw.statusCode >= 500 || crw.statusCode == 0 {
		cb.RecordFailure()
		cb.log.Debug("Circuit breaker recorded failure",
			logger.String("circuit", cb.name),
			logger.String("path", req.URL.Path),
			logger.Int("status_code", crw.statusCode),
			logger.Int("failures", cb.failures))
	} else {
		cb.RecordSuccess()
		cb.log.Debug("Circuit breaker recorded success",
			logger.String("circuit", cb.name),
			logger.String("path", req.URL.Path),
			logger.Int("status_code", crw.statusCode))
	}

	return nil
}

// AllowRequest checks if a request should be allowed based on circuit state
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mutex.RLock()
	state := cb.state
	lastFailure := cb.lastFailure
	cb.mutex.RUnlock()

	switch state {
	case Closed:
		// Circuit is closed, requests are allowed
		return true
	case Open:
		// Check if timeout has elapsed to try a test request
		timeout := cb.config.Timeout
		elapsed := time.Since(lastFailure)

		if elapsed > timeout {
			cb.mutex.Lock()
			// Double-check the state in case another goroutine changed it
			if cb.state == Open {
				cb.state = HalfOpen
				cb.log.Info("Circuit breaker transitioned to half-open",
					logger.String("circuit", cb.name),
					logger.String("elapsed", elapsed.String()),
					logger.String("timeout", timeout.String()),
				)
			}
			cb.mutex.Unlock()
			return true
		}

		cb.log.Debug("Circuit breaker is open, rejecting request",
			logger.String("circuit", cb.name),
			logger.String("elapsed", elapsed.String()),
			logger.String("timeout", timeout.String()),
		)
		return false
	case HalfOpen:
		// In half-open state, allow only one request for testing
		return true
	default:
		return true
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.totalRequests++

	switch cb.state {
	case HalfOpen:
		// If successful in half-open state, close the circuit
		cb.failures = 0
		cb.state = Closed
		cb.log.Info("Circuit breaker closed after successful test request",
			logger.String("circuit", cb.name),
			logger.Int("total_requests", cb.totalRequests),
			logger.Int("total_failures", cb.totalFailures),
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

	cb.totalRequests++
	cb.totalFailures++
	cb.lastFailure = time.Now()

	switch cb.state {
	case HalfOpen:
		// If failed in half-open state, open the circuit again
		cb.state = Open
		cb.log.Warn("Circuit breaker reopened after failed test request",
			logger.String("circuit", cb.name),
			logger.Int("total_requests", cb.totalRequests),
			logger.Int("total_failures", cb.totalFailures),
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
				logger.Int("threshold", cb.config.Threshold),
				logger.Int("total_requests", cb.totalRequests),
				logger.Int("total_failures", cb.totalFailures),
			)
		} else {
			cb.log.Debug("Circuit breaker failure count increased",
				logger.String("circuit", cb.name),
				logger.Int("failures", cb.failures),
				logger.Int("threshold", cb.config.Threshold),
			)
		}
	}
}

// GetStatus returns the current state and metrics of the circuit breaker
func (cb *CircuitBreaker) GetStatus() map[string]interface{} {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return map[string]interface{}{
		"name":           cb.name,
		"state":          cb.state.String(),
		"failures":       cb.failures,
		"threshold":      cb.config.Threshold,
		"total_requests": cb.totalRequests,
		"total_failures": cb.totalFailures,
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

// Write captures the response body and ensures status code is set
func (crw *customResponseWriter) Write(b []byte) (int, error) {
	// If WriteHeader hasn't been called yet, set the status to 200 OK
	if crw.statusCode == 0 {
		crw.statusCode = http.StatusOK
	}
	return crw.ResponseWriter.Write(b)
}
