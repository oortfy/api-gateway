package middleware

import (
	"context"
	"net/http"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.16.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware provides distributed tracing functionality
type TracingMiddleware struct {
	config      *config.TracingConfig
	log         logger.Logger
	tracer      trace.Tracer
	tp          *sdktrace.TracerProvider
	initialized bool
}

// NewTracingMiddleware creates a new tracing middleware
func NewTracingMiddleware(config *config.TracingConfig, log logger.Logger) *TracingMiddleware {
	tm := &TracingMiddleware{
		config: config,
		log:    log,
	}

	// Only initialize if tracing is enabled
	if config.Enabled {
		if err := tm.initialize(); err != nil {
			log.Error("Failed to initialize tracing", logger.Error(err))
		}
	}

	return tm
}

// Initialize sets up the tracer provider
func (t *TracingMiddleware) initialize() error {
	// Create Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(t.config.Endpoint)))
	if err != nil {
		return err
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(t.config.ServiceName),
		)),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(t.config.SampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create tracer
	t.tracer = tp.Tracer("api-gateway")
	t.tp = tp
	t.initialized = true

	t.log.Info("Tracing initialized",
		logger.String("provider", t.config.Provider),
		logger.String("endpoint", t.config.Endpoint),
		logger.String("service", t.config.ServiceName),
		logger.Any("sample_rate", t.config.SampleRate),
	)

	return nil
}

// Tracing middleware adds distributed tracing to requests
func (t *TracingMiddleware) Tracing(next http.Handler) http.Handler {
	if !t.config.Enabled || !t.initialized {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract context from request headers
		ctx := r.Context()
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))

		// Start a new span
		spanName := r.Method + " " + r.URL.Path
		ctx, span := t.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		// Add attributes to the span
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.host", r.Host),
			attribute.String("http.user_agent", r.UserAgent()),
		)

		// Create a response writer that captures the status code
		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Process the request with the new context
		next.ServeHTTP(recorder, r.WithContext(ctx))

		// Add status code to span
		span.SetAttributes(attribute.Int("http.status_code", recorder.statusCode))

		// Mark as error if status code is 5xx
		if recorder.statusCode >= 500 {
			span.SetAttributes(attribute.Bool("error", true))
		}
	})
}

// Shutdown cleanly shuts down the tracer provider
func (t *TracingMiddleware) Shutdown(ctx context.Context) error {
	if t.config.Enabled && t.initialized && t.tp != nil {
		return t.tp.Shutdown(ctx)
	}
	return nil
}
