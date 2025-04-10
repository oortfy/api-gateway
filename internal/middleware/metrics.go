package middleware

import (
	"net/http"
	"strconv"
	"time"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// RequestDuration tracks request duration
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// RequestsTotal tracks the total number of requests
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_requests_total",
			Help: "Total number of requests",
		},
		[]string{"method", "path", "status"},
	)

	// CircuitBreakerStatus tracks circuit breaker status
	circuitBreakerStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_circuit_breaker_status",
			Help: "Circuit breaker status (0=closed, 1=open, 2=half-open)",
		},
		[]string{"path"},
	)

	// CacheHits tracks cache hits
	cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"path"},
	)

	// CacheMisses tracks cache misses
	cacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"path"},
	)

	// RateLimitRejections tracks rate limit rejections
	rateLimitRejections = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_rate_limit_rejections_total",
			Help: "Total number of requests rejected due to rate limits",
		},
		[]string{"path"},
	)
)

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(circuitBreakerStatus)
	prometheus.MustRegister(cacheHits)
	prometheus.MustRegister(cacheMisses)
	prometheus.MustRegister(rateLimitRejections)
}

// MetricsMiddleware provides metrics collection and endpoints
type MetricsMiddleware struct {
	config *config.MetricsConfig
	log    logger.Logger
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(config *config.MetricsConfig, log logger.Logger) *MetricsMiddleware {
	return &MetricsMiddleware{
		config: config,
		log:    log,
	}
}

// RegisterMetricsEndpoint registers the metrics endpoint
func (m *MetricsMiddleware) RegisterMetricsEndpoint(router http.Handler) http.Handler {
	if !m.config.Enabled {
		return router
	}

	// Create a handler for the metrics endpoint
	handler := http.NewServeMux()

	// Copy all requests to the original router
	handler.Handle("/", router)

	// Add the metrics endpoint
	handler.Handle(m.config.Endpoint, promhttp.Handler())

	m.log.Info("Registered metrics endpoint",
		logger.String("endpoint", m.config.Endpoint),
	)

	return handler
}

// Metrics middleware collects metrics for each request
func (m *MetricsMiddleware) Metrics(next http.Handler) http.Handler {
	if !m.config.Enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer that captures the status code
		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Process the request
		next.ServeHTTP(recorder, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		path := r.URL.Path
		method := r.Method
		status := strconv.Itoa(recorder.statusCode)

		requestDuration.WithLabelValues(method, path, status).Observe(duration)
		requestsTotal.WithLabelValues(method, path, status).Inc()
	})
}

// IncrementCacheHit increments the cache hit counter
func (m *MetricsMiddleware) IncrementCacheHit(path string) {
	if m.config.Enabled {
		cacheHits.WithLabelValues(path).Inc()
	}
}

// IncrementCacheMiss increments the cache miss counter
func (m *MetricsMiddleware) IncrementCacheMiss(path string) {
	if m.config.Enabled {
		cacheMisses.WithLabelValues(path).Inc()
	}
}

// IncrementRateLimit increments the rate limit counter
func (m *MetricsMiddleware) IncrementRateLimit(path string) {
	if m.config.Enabled {
		rateLimitRejections.WithLabelValues(path).Inc()
	}
}

// SetCircuitBreakerStatus sets the circuit breaker status
func (m *MetricsMiddleware) SetCircuitBreakerStatus(path string, status float64) {
	if m.config.Enabled {
		circuitBreakerStatus.WithLabelValues(path).Set(status)
	}
}
