package testutils

import (
	"api-gateway/pkg/logger"

	"context"

	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// MockLogger is a mock implementation of the logger.Logger interface
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...logger.Field) {
	args := m.Called(msg, fields)
	_ = args
}

func (m *MockLogger) Info(msg string, fields ...logger.Field) {
	args := m.Called(msg, fields)
	_ = args
}

func (m *MockLogger) Warn(msg string, fields ...logger.Field) {
	args := m.Called(msg, fields)
	_ = args
}

func (m *MockLogger) Error(msg string, fields ...logger.Field) {
	args := m.Called(msg, fields)
	_ = args
}

func (m *MockLogger) Fatal(msg string, fields ...logger.Field) {
	args := m.Called(msg, fields)
	_ = args
}

func (m *MockLogger) With(fields ...logger.Field) logger.Logger {
	args := m.Called(fields)
	if ret := args.Get(0); ret != nil {
		return ret.(logger.Logger)
	}
	return m
}

// MockSpan is a mock implementation of the trace.Span interface
type MockSpan struct {
	mock.Mock
}

func (m *MockSpan) End(options ...trace.SpanEndOption) {
	m.Called(options)
}

func (m *MockSpan) AddEvent(name string, options ...trace.EventOption) {
	m.Called(name, options)
}

func (m *MockSpan) IsRecording() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockSpan) RecordError(err error, options ...trace.EventOption) {
	m.Called(err, options)
}

func (m *MockSpan) SpanContext() trace.SpanContext {
	args := m.Called()
	return args.Get(0).(trace.SpanContext)
}

func (m *MockSpan) SetStatus(code codes.Code, description string) {
	m.Called(code, description)
}

func (m *MockSpan) SetName(name string) {
	m.Called(name)
}

func (m *MockSpan) SetAttributes(attrs ...attribute.KeyValue) {
	m.Called(attrs)
}

func (m *MockSpan) TracerProvider() trace.TracerProvider {
	args := m.Called()
	return args.Get(0).(trace.TracerProvider)
}

// MockTracer is a mock implementation of the trace.Tracer interface
type MockTracer struct {
	mock.Mock
}

func (m *MockTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	args := m.Called(ctx, spanName, opts)
	return args.Get(0).(context.Context), args.Get(1).(trace.Span)
}

// MockTracerProvider is a mock implementation of the trace.TracerProvider interface
type MockTracerProvider struct {
	mock.Mock
}

func (m *MockTracerProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	args := m.Called(name, opts)
	return args.Get(0).(trace.Tracer)
}
