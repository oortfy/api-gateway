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

## Configuration

The API Gateway is configured using two YAML files:

1. `config.yaml`: Contains general configuration for the server, authentication, and logging.
2. `routes.yaml`: Defines the routing rules for upstream services.

These files can be located anywhere on the filesystem and specified using either command-line flags or environment variables.

### Environment Variables

- `JWT_SECRET`: Secret key used for JWT validation (required if using JWT auth)
- `CONFIG_PATH`: Path to the config.yaml file (default: configs/config.yaml)
- `ROUTES_PATH`: Path to the routes.yaml file (default: configs/routes.yaml)

## Quick Start

### Prerequisites

- Go 1.16 or higher
- Docker (optional)

### Running Locally

1. Clone the repository

2. Set the required environment variables
   ```
   export JWT_SECRET=your_jwt_secret_here
   export CONFIG_PATH=/path/to/config.yaml
   export ROUTES_PATH=/path/to/routes.yaml
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
     -v $(pwd)/configs:/app/configs \
     api-gateway
   ```

### Using Docker Compose

1. Run with docker-compose
   ```
   docker-compose up -d
   ```

   This will mount the local `configs` directory into the container, allowing you to update the configuration files without rebuilding the image.

## Route Configuration

Routes are defined in `routes.yaml`. Here's an example:

```yaml
routes:
  - path: "/api/users"
    methods: ["GET", "POST", "PUT", "DELETE"]
    upstream: "http://user-service:8082"
    strip_prefix: true
    require_auth: true
    allowed_roles: ["admin", "user"]
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
2. API keys (via the `X-API-Auth-Token` header)

Each route can be configured to require authentication and restrict access to specific roles.

## Health Check

The API Gateway provides a health check endpoint at `/health` that returns the current status of the service.

## License

This project is licensed under the MIT License - see the LICENSE file for details. 