package middleware

import (
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

// mockMetricsLogger for testing
type mockMetricsLogger struct{}

func (m *mockMetricsLogger) Debug(msg string, fields ...logger.Field)  {}
func (m *mockMetricsLogger) Info(msg string, fields ...logger.Field)   {}
func (m *mockMetricsLogger) Warn(msg string, fields ...logger.Field)   {}
func (m *mockMetricsLogger) Error(msg string, fields ...logger.Field)  {}
func (m *mockMetricsLogger) Fatal(msg string, fields ...logger.Field)  {}
func (m *mockMetricsLogger) With(fields ...logger.Field) logger.Logger { return m }

// getMetricValue is a helper function to get the value of a Prometheus metric
func getMetricValue(c prometheus.Collector, labels map[string]string) (float64, error) {
	var m dto.Metric
	var metric prometheus.Metric

	// For counter or gauge metrics
	if counter, ok := c.(prometheus.Counter); ok {
		metric = counter
	} else if gauge, ok := c.(prometheus.Gauge); ok {
		metric = gauge
	} else if counterVec, ok := c.(*prometheus.CounterVec); ok {
		metric, _ = counterVec.GetMetricWith(labels)
	} else if gaugeVec, ok := c.(*prometheus.GaugeVec); ok {
		metric, _ = gaugeVec.GetMetricWith(labels)
	} else if histogramVec, ok := c.(*prometheus.HistogramVec); ok {
		// For histogram metrics, we'll return the count
		observer, _ := histogramVec.GetMetricWith(labels)
		metric = observer.(prometheus.Metric)
	}

	if metric == nil {
		return 0, nil
	}

	if err := metric.Write(&m); err != nil {
		return 0, err
	}

	if m.Counter != nil {
		return m.Counter.GetValue(), nil
	} else if m.Gauge != nil {
		return m.Gauge.GetValue(), nil
	} else if m.Histogram != nil {
		return float64(m.Histogram.GetSampleCount()), nil
	}

	return 0, nil
}

func TestNewMetricsMiddleware(t *testing.T) {
	config := &config.MetricsConfig{
		Enabled:  true,
		Endpoint: "/metrics",
	}
	log := &mockMetricsLogger{}

	middleware := NewMetricsMiddleware(config, log)

	assert.NotNil(t, middleware)
	assert.Equal(t, config, middleware.config)
	assert.Equal(t, log, middleware.log)
}

func TestMetricsMiddleware_RegisterMetricsEndpoint_Enabled(t *testing.T) {
	config := &config.MetricsConfig{
		Enabled:  true,
		Endpoint: "/metrics",
	}
	log := &mockMetricsLogger{}

	middleware := NewMetricsMiddleware(config, log)

	// Create a basic router
	router := http.NewServeMux()
	router.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Register metrics endpoint
	handler := middleware.RegisterMetricsEndpoint(router)

	// Test the regular endpoint still works
	req1 := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.Equal(t, "OK", rec1.Body.String())

	// Test the metrics endpoint
	req2 := httptest.NewRequest("GET", "http://example.com/metrics", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	// Should respond with Prometheus metrics
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "# HELP ") // Basic check for Prometheus metrics format
}

func TestMetricsMiddleware_RegisterMetricsEndpoint_Disabled(t *testing.T) {
	config := &config.MetricsConfig{
		Enabled:  false,
		Endpoint: "/metrics",
	}
	log := &mockMetricsLogger{}

	middleware := NewMetricsMiddleware(config, log)

	// Create a basic router
	router := http.NewServeMux()
	router.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// No metrics endpoint should be registered when disabled
	handler := middleware.RegisterMetricsEndpoint(router)

	// Test the regular endpoint still works
	req1 := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.Equal(t, "OK", rec1.Body.String())

	// Test the metrics endpoint - should not be registered
	req2 := httptest.NewRequest("GET", "http://example.com/metrics", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	// Should get 404 as the metrics endpoint is not registered
	assert.Equal(t, http.StatusNotFound, rec2.Code)
}

func TestMetricsMiddleware_Metrics_Enabled(t *testing.T) {
	// Reset metrics registry to get clean metrics
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	prometheus.DefaultGatherer = prometheus.DefaultRegisterer.(prometheus.Gatherer)

	// Re-register metrics
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(requestsTotal)

	config := &config.MetricsConfig{
		Enabled:  true,
		Endpoint: "/metrics",
	}
	log := &mockMetricsLogger{}

	middleware := NewMetricsMiddleware(config, log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with metrics middleware
	handler := middleware.Metrics(testHandler)

	// Send a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())

	// Check that metrics were recorded
	// For the total requests metric
	metricCount, err := getMetricValue(requestsTotal, map[string]string{
		"method": "GET",
		"path":   "/api/test",
		"status": "200",
	})
	assert.NoError(t, err)
	assert.Equal(t, float64(1), metricCount, "request count metric should be incremented")

	// For the duration metric, we just check it exists since exact timing is variable
	metricValue, err := getMetricValue(requestDuration, map[string]string{
		"method": "GET",
		"path":   "/api/test",
		"status": "200",
	})
	assert.NoError(t, err)
	assert.Greater(t, metricValue, float64(0), "duration metric should be positive")
}

func TestMetricsMiddleware_Metrics_Disabled(t *testing.T) {
	// Reset metrics registry to get clean metrics
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	prometheus.DefaultGatherer = prometheus.DefaultRegisterer.(prometheus.Gatherer)

	// Re-register metrics
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(requestsTotal)

	config := &config.MetricsConfig{
		Enabled:  false,
		Endpoint: "/metrics",
	}
	log := &mockMetricsLogger{}

	middleware := NewMetricsMiddleware(config, log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with metrics middleware (disabled)
	handler := middleware.Metrics(testHandler)

	// Send a request
	req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}

func TestMetricsMiddleware_IncrementCacheHit(t *testing.T) {
	// Reset metrics registry to get clean metrics
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	prometheus.DefaultGatherer = prometheus.DefaultRegisterer.(prometheus.Gatherer)

	// Re-register the metric we'll test
	prometheus.MustRegister(cacheHits)

	config := &config.MetricsConfig{
		Enabled:  true,
		Endpoint: "/metrics",
	}
	log := &mockMetricsLogger{}

	middleware := NewMetricsMiddleware(config, log)

	// Path to record cache hit for
	path := "/api/cached"

	// Initial value should be 0
	initial, _ := getMetricValue(cacheHits, map[string]string{"path": path})
	assert.Equal(t, float64(0), initial)

	// Increment cache hit counter
	middleware.IncrementCacheHit(path)

	// Value should now be 1
	after, _ := getMetricValue(cacheHits, map[string]string{"path": path})
	assert.Equal(t, float64(1), after)
}

func TestMetricsMiddleware_IncrementCacheMiss(t *testing.T) {
	// Reset metrics registry to get clean metrics
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	prometheus.DefaultGatherer = prometheus.DefaultRegisterer.(prometheus.Gatherer)

	// Re-register the metric we'll test
	prometheus.MustRegister(cacheMisses)

	config := &config.MetricsConfig{
		Enabled:  true,
		Endpoint: "/metrics",
	}
	log := &mockMetricsLogger{}

	middleware := NewMetricsMiddleware(config, log)

	// Path to record cache miss for
	path := "/api/missed"

	// Initial value should be 0
	initial, _ := getMetricValue(cacheMisses, map[string]string{"path": path})
	assert.Equal(t, float64(0), initial)

	// Increment cache miss counter
	middleware.IncrementCacheMiss(path)

	// Value should now be 1
	after, _ := getMetricValue(cacheMisses, map[string]string{"path": path})
	assert.Equal(t, float64(1), after)
}

func TestMetricsMiddleware_IncrementRateLimit(t *testing.T) {
	// Reset metrics registry to get clean metrics
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	prometheus.DefaultGatherer = prometheus.DefaultRegisterer.(prometheus.Gatherer)

	// Re-register the metric we'll test
	prometheus.MustRegister(rateLimitRejections)

	config := &config.MetricsConfig{
		Enabled:  true,
		Endpoint: "/metrics",
	}
	log := &mockMetricsLogger{}

	middleware := NewMetricsMiddleware(config, log)

	// Path to record rate limit for
	path := "/api/limited"

	// Initial value should be 0
	initial, _ := getMetricValue(rateLimitRejections, map[string]string{"path": path})
	assert.Equal(t, float64(0), initial)

	// Increment rate limit counter
	middleware.IncrementRateLimit(path)

	// Value should now be 1
	after, _ := getMetricValue(rateLimitRejections, map[string]string{"path": path})
	assert.Equal(t, float64(1), after)
}

func TestMetricsMiddleware_SetCircuitBreakerStatus(t *testing.T) {
	// Reset metrics registry to get clean metrics
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	prometheus.DefaultGatherer = prometheus.DefaultRegisterer.(prometheus.Gatherer)

	// Re-register the metric we'll test
	prometheus.MustRegister(circuitBreakerStatus)

	config := &config.MetricsConfig{
		Enabled:  true,
		Endpoint: "/metrics",
	}
	log := &mockMetricsLogger{}

	middleware := NewMetricsMiddleware(config, log)

	// Path to set circuit breaker status for
	path := "/api/circuit"

	// Initial value should be 0
	initial, _ := getMetricValue(circuitBreakerStatus, map[string]string{"path": path})
	assert.Equal(t, float64(0), initial)

	// Set circuit breaker status to "open" (1)
	middleware.SetCircuitBreakerStatus(path, 1)

	// Value should now be 1
	open, _ := getMetricValue(circuitBreakerStatus, map[string]string{"path": path})
	assert.Equal(t, float64(1), open)

	// Set circuit breaker status to "half-open" (2)
	middleware.SetCircuitBreakerStatus(path, 2)

	// Value should now be 2
	halfOpen, _ := getMetricValue(circuitBreakerStatus, map[string]string{"path": path})
	assert.Equal(t, float64(2), halfOpen)

	// Set circuit breaker status back to "closed" (0)
	middleware.SetCircuitBreakerStatus(path, 0)

	// Value should now be 0
	closed, _ := getMetricValue(circuitBreakerStatus, map[string]string{"path": path})
	assert.Equal(t, float64(0), closed)
}

func TestMetricsMiddleware_MetricsDisabled(t *testing.T) {
	// Reset metrics registry to get clean metrics
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	prometheus.DefaultGatherer = prometheus.DefaultRegisterer.(prometheus.Gatherer)

	// Re-register metrics
	prometheus.MustRegister(cacheHits)
	prometheus.MustRegister(cacheMisses)
	prometheus.MustRegister(rateLimitRejections)
	prometheus.MustRegister(circuitBreakerStatus)

	config := &config.MetricsConfig{
		Enabled:  false, // Metrics disabled
		Endpoint: "/metrics",
	}
	log := &mockMetricsLogger{}

	middleware := NewMetricsMiddleware(config, log)

	// Path for metrics
	path := "/api/test"

	// Call all metrics methods
	middleware.IncrementCacheHit(path)
	middleware.IncrementCacheMiss(path)
	middleware.IncrementRateLimit(path)
	middleware.SetCircuitBreakerStatus(path, 1)

	// All metrics should still be 0 as metrics are disabled
	cacheHitValue, _ := getMetricValue(cacheHits, map[string]string{"path": path})
	cacheMissValue, _ := getMetricValue(cacheMisses, map[string]string{"path": path})
	rateLimitValue, _ := getMetricValue(rateLimitRejections, map[string]string{"path": path})
	circuitBreakerValue, _ := getMetricValue(circuitBreakerStatus, map[string]string{"path": path})

	assert.Equal(t, float64(0), cacheHitValue)
	assert.Equal(t, float64(0), cacheMissValue)
	assert.Equal(t, float64(0), rateLimitValue)
	assert.Equal(t, float64(0), circuitBreakerValue)
}
