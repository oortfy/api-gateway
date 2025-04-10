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

## Quick Start

### Prerequisites

- Go 1.16 or higher
- Docker (optional)

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

### Using Docker Compose

1. Edit the docker-compose.yml file to set your environment variables:
   ```yaml
   services:
     api-gateway:
       # ... other settings ...
       environment:
         - JWT_SECRET=your_jwt_secret_here
         - API_VALIDATION_URL=http://auth-service:8081/auth/validate-api-key
         - LOG_LEVEL=info
   ```

2. Run with docker-compose
   ```
   docker-compose up -d
   ```

   This will mount the local `configs` directory into the container, allowing you to update the configuration files without rebuilding the image.

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

## Route Configuration

Routes are defined in `routes.yaml`. Here's an example:

```yaml
routes:
  - path: "/api/users"
    methods: ["GET", "POST", "PUT", "DELETE"]
    upstream: "http://user-service:8082"
    strip_prefix: true
    require_auth: true
    timeout: 30
```

### Advanced Route Configuration

The API Gateway supports several advanced features that can be configured per route:

#### Caching

Enable response caching for improved performance:

```yaml
  - path: "/api/products"
    upstream: "http://product-service:8083"
    methods: ["GET"]
    cache:
      enabled: true
      ttl: 300  # Cache TTL in seconds
      cache_authenticated: false  # Whether to cache authenticated requests
```

The cache middleware stores responses in memory and serves them for GET requests when available, respecting the configured TTL.

#### Circuit Breaker

Protect upstream services from cascading failures:

```yaml
  - path: "/api/orders"
    upstream: "http://order-service:8084"
    circuit_breaker:
      enabled: true
      threshold: 5  # Number of failures before opening
      timeout: 30   # Seconds before attempting to close
      max_concurrent: 100  # Maximum concurrent requests
```

The circuit breaker pattern prevents overwhelming a struggling service by temporarily refusing connections after detecting failures.

#### Rate Limiting

Control request rates to protect upstream services:

```yaml
  - path: "/api/search"
    upstream: "http://search-service:8085"
    rate_limit:
      requests: 100  # Maximum requests
      period: "1m"   # Time period (s, m, h)
```

#### Header Transformation

Modify request and response headers:

```yaml
  - path: "/api/legacy"
    upstream: "http://legacy-service:8087"
    header_transform:
      request:
        "X-Source": "api-gateway"
      response:
        "Access-Control-Allow-Origin": "*"
      remove: ["X-Powered-By"]
```

#### URL Rewriting

Rewrite URLs before forwarding to upstream services:

```yaml
  - path: "/api/v2"
    upstream: "http://service:8088"
    url_rewrite:
      patterns:
        - match: "/api/v2/users/(.*)"
          replacement: "/internal/users/$1"
```

#### Retry Policy

Configure retry behavior for failed requests:

```yaml
  - path: "/api/notifications"
    upstream: "http://notification-service:8089"
    retry_policy:
      enabled: true
      attempts: 3
      per_try_timeout: 5
      retry_on: ["connection_error", "server_error"]
```

### WebSocket Configuration

WebSocket routes can be configured with the `websocket` property:

```yaml
  - path: "/ws"
    upstream: "http://websocket-service:8086"
    require_auth: true
    websocket:
      enabled: true
      path: "/ws"
      upstream_path: "/socket"
```

## Authentication

The API Gateway supports two authentication methods:

1. JWT tokens (via the `Authorization` header with the Bearer scheme)
2. API keys (via the `x-api-key` header)

Each route can be configured to require authentication with the `require_auth` setting.

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