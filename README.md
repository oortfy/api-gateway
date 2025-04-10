# API Gateway

A high-performance API Gateway written in Go that supports REST and WebSocket proxying with JWT and API key authentication.

## Features

- REST API routing and proxying
- WebSocket support
- Authentication with JWT and API tokens
- Route configuration via YAML
- Structured logging
- High availability and performance
- Graceful shutdown
- Docker support
- External configuration files
- Environment-based configuration
- Response caching with TTL
- Circuit breaker pattern for fault tolerance
- Rate limiting
- Header transformations
- URL rewriting
- Load balancing
- Retry policies

## Configuration

The API Gateway is configured using two YAML files:

1. `config.yaml`: Contains general configuration for the server, authentication, and logging.
2. `routes.yaml`: Defines the routing rules for upstream services.

These files can be located anywhere on the filesystem and specified using either command-line flags or environment variables.

### Environment Variables

All sensitive configuration values use environment variables to avoid hardcoding credentials:

**Required Environment Variables:**
- `JWT_SECRET`: Secret key used for JWT validation
- `API_VALIDATION_URL`: URL for validating API keys (e.g., "http://auth-service:8081/auth/validate-api-key")

**Optional Environment Variables:**
- `CONFIG_PATH`: Path to the config.yaml file (default: configs/config.yaml)
- `ROUTES_PATH`: Path to the routes.yaml file (default: configs/routes.yaml)
- `LOG_LEVEL`: Logging level (default: info)
- `LOG_FORMAT`: Logging format (default: json)
- `TRACING_ENDPOINT`: URL for sending traces (default: http://jaeger:14268/api/traces)

## Quick Start

### Prerequisites

- Go 1.16 or higher
- Docker and Docker Compose (for full testing environment)

### Running Locally

1. Clone the repository

2. Set the required environment variables
   ```
   export JWT_SECRET=your_jwt_secret_here
   export API_VALIDATION_URL=http://auth-service:8081/auth/validate-api-key
   export CONFIG_PATH=/path/to/config.yaml
   export ROUTES_PATH=/path/to/routes.yaml
   export LOG_LEVEL=debug  # Optional
   export LOG_FORMAT=json  # Optional
   ```

3. Run the application
   ```
   go run cmd/api/main.go
   ```

### Using Docker

1. Build the Docker image
   ```
   docker build -t api-gateway .
   ```

2. Run the container with mounted config files
   ```
   docker run -p 8080:8080 \
     -e JWT_SECRET=your_jwt_secret_here \
     -e API_VALIDATION_URL=http://auth-service:8081/auth/validate-api-key \
     -e LOG_LEVEL=info \
     -v $(pwd)/configs:/app/configs \
     api-gateway
   ```

### Using Docker Compose (Recommended for Testing)

The included Docker Compose setup provides a complete environment for testing all features of the API Gateway, including mock services for each route.

1. Start the environment with mock services:
   ```
   docker-compose up -d
   ```

2. To view logs:
   ```
   docker-compose logs -f api-gateway
   ```

3. To stop the environment:
   ```
   docker-compose down
   ```

## Testing All Features

After starting the Docker Compose environment, you can test the various features of the API Gateway:

### 1. Basic Routing

Test basic routing to the user service:
```bash
curl -X GET http://localhost:8080/api/users
```

### 2. Authentication

Test authentication (API Key):
```bash
curl -X GET http://localhost:8080/api/users -H "X-API-Key: test-api-key"
```

Test authentication (JWT - requires a valid JWT token):
```bash
curl -X GET http://localhost:8080/api/users -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

To generate a test JWT token:
```bash
# Install jwt-cli if needed
# npm install -g jwt-cli
jwt sign --secret your_jwt_secret_here '{"sub": "test-user", "name": "Test User", "role": "admin"}'
```

### 3. Caching

Test caching by making repeated requests to the product service:
```bash
curl -X GET http://localhost:8080/api/products -H "X-API-Key: test-api-key"
```

You should see faster response times on subsequent requests.

### 4. Rate Limiting

Test rate limiting by making multiple rapid requests to the search endpoint:
```bash
for i in {1..60}; do curl -X GET http://localhost:8080/api/search; done
```

After 50 requests, you should start receiving 429 Too Many Requests responses.

### 5. Circuit Breaker

The circuit breaker can be tested by causing failures in the mock services and observing how the API Gateway responds.

### 6. WebSocket

To test WebSocket functionality, you can use the `websocat` tool:
```bash
# Install websocat if needed
# brew install websocat (macOS) or cargo install websocat (with Rust)
websocat ws://localhost:8080/ws
```

### 7. Health Check

Test the health check endpoint:
```bash
curl -X GET http://localhost:8080/health
```

### 8. Tracing

After using the API Gateway, you can view traces in the Jaeger UI:
1. Open http://localhost:16686 in your browser
2. Select "api-gateway" from the Service dropdown
3. Click "Find Traces" to see trace information

### 9. Header Transformation

Test header transformation:
```bash
curl -X GET http://localhost:8080/api/products -H "X-API-Key: test-api-key" -v
```

Observe the response headers to see the added Cache-Control header.

### 10. URL Rewriting

Test URL rewriting:
```bash
curl -X GET http://localhost:8080/api/legacy/users/123 -H "X-API-Key: test-api-key" -v
```

This will rewrite the path to `/internal/users/123` before forwarding to the legacy service.

## Configuration Templates

The config.yaml file supports environment variable substitution using the `${VARIABLE_NAME}` syntax. For example:

```yaml
auth:
  jwt_secret: "${JWT_SECRET}"
  api_key_validation_url: "${API_VALIDATION_URL}"
```

You can also specify default values for optional variables:

```yaml
logging:
  level: "${LOG_LEVEL:-info}"
  format: "${LOG_FORMAT:-json}"
```

## Troubleshooting

### Common Issues

1. **Connection Refused**: Make sure all required services are running.
2. **Authentication Errors**: Verify that the JWT_SECRET environment variable is set correctly.
3. **Missing Routes**: Check the routes.yaml file for correct path, methods, and upstream values.
4. **Docker Network Issues**: Ensure services can communicate by checking they're on the same network.

### Checking Logs

To check logs of any service:
```bash
docker-compose logs -f [service-name]
```

Example:
```bash
docker-compose logs -f api-gateway
docker-compose logs -f auth-service
```

For all services:
```bash
docker-compose logs -f
```

## Architecture

The API Gateway is structured with clean architecture principles:

```
api-gateway/
├── cmd/            # Application entry points
├── configs/        # Configuration files
├── internal/       # Private application code
│   ├── auth/       # Authentication logic
│   ├── config/     # Configuration loading and parsing
│   ├── handlers/   # HTTP handlers
│   ├── middleware/ # HTTP middleware (auth, cache, etc.)
│   ├── models/     # Data models
│   ├── proxy/      # Proxy implementation (HTTP, WebSocket, circuit breaker)
│   └── server/     # HTTP server implementation
└── pkg/            # Public libraries
    └── logger/     # Structured logging
```

## Health Check

The API Gateway provides a health check endpoint at `/health` that returns the current status of the service.

## License

This project is licensed under the MIT License - see the LICENSE file for details. 