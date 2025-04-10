# API Gateway Tests

This directory contains tests for the API Gateway project, organized by component.

## Directory Structure

```
tests/
├── middleware/         # Tests for middleware components
│   ├── metrics_test.go    # Tests for metrics middleware
│   ├── ratelimit_test.go  # Tests for rate limiter middleware
│   └── tracing_test.go    # Tests for tracing middleware
└── testutils/          # Common test utilities and mocks
    └── mocks.go        # Shared mock implementations
```

## Running Tests

To run all tests:

```bash
go test -v ./tests/...
```

To run tests for a specific package:

```bash
go test -v ./tests/middleware/...
```

To run a specific test:

```bash
go test -v ./tests/middleware -run TestTracingMiddleware_Enabled
```

To skip long-running tests:

```bash
go test -v -short ./tests/...
```

## Test Coverage

To check test coverage:

```bash
go test -cover ./tests/...
```

For a detailed coverage report:

```bash
go test -coverprofile=coverage.out ./tests/...
go tool cover -html=coverage.out
```

## Adding New Tests

When adding new tests:

1. Create a test file in the appropriate subdirectory
2. Use the `_test` package suffix to test from an external perspective
3. Leverage shared mocks from the `testutils` package
4. Follow the existing test patterns
5. Ensure you handle edge cases and error conditions

## Mocks

Common mock implementations are provided in the `testutils/mocks.go` file, including:

- `MockLogger` - Mock implementation of the logger.Logger interface
- `MockSpan` - Mock implementation of the trace.Span interface
- `MockTracer` - Mock implementation of the trace.Tracer interface
- `MockTracerProvider` - Mock implementation of the trace.TracerProvider interface

When adding new tests that require mocking, consider adding reusable mocks to the `testutils` package. 