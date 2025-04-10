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

## Health Check

The API Gateway provides a health check endpoint at `/health` that returns the current status of the service.

## License

This project is licensed under the MIT License - see the LICENSE file for details. 