# Oortfy API Gateway

A high-performance, modular, and configuration-driven API Gateway built in Go, designed for modern microservices architectures. It provides advanced traffic management, security, observability, and developer experience features.

[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?style=flat&logo=go)](https://golang.org/doc/devel/release.html)
[![License](https://img.shields.io/badge/License-MPL%202.0-blue.svg)](LICENSE)

---

## 🚀 Features

- **Dynamic Routing & Proxy**
  - HTTP and WebSocket proxying
  - Path-based routing, prefix stripping, and URL rewriting
  - Modular per-route middleware configuration

- **Traffic Management**
  - Rate limiting (per route, per client)
  - Circuit breaker
  - Request retries with backoff
  - Load balancing (static, service discovery)
  - Response caching (configurable per route)

- **Security**
  - API Key and JWT authentication (header or query param)
  - CORS configuration
  - TLS support
  - Header security (HSTS, XSS, etc.)

- **Observability**
  - Prometheus metrics
  - Distributed tracing (Jaeger, OpenTelemetry)
  - Structured JSON logging
  - Health checks
  - Optional IP geolocation (IP2Location LITE)

- **Developer Experience**
  - 📚 **Dynamic OpenAPI/Swagger documentation** auto-generated from your route config
  - Hot-reload ready (config-driven)
  - Easy local development and testing

- **Protocol Support**
  - **HTTP Proxying**: Traditional HTTP/HTTPS reverse proxy
  - **gRPC Support**: Full gRPC support with multiple operation modes:
    - Pure gRPC proxying (gRPC → gRPC)
    - Protocol conversion (HTTP ↔ gRPC)
    - Automatic service discovery via etcd
    - Support for gRPC reflection
    - Streaming support
    - Load balancing for gRPC services

## 📋 Table of Contents
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Route Examples](#route-examples)
- [Authentication](#authentication)
- [Traffic Management](#traffic-management)
- [Observability](#observability)
- [Client IP & Geolocation](#client-ip--geolocation)
- [API Documentation](#api-documentation)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

---

## 🚀 Quick Start

### Prerequisites
- Go 1.20+
- Docker & Docker Compose
- Make (optional)

### Installation
```bash
# Clone the repository
git clone https://github.com/yourusername/api-gateway.git
cd api-gateway

# Build and run with Docker
docker-compose up -d

# Or build and run locally
make build
./bin/api-gateway
```

### Basic Usage
1. Configure your routes in `configs/routes.yaml`:
```yaml
routes:
  - path: "/auth/*"
    upstream: "http://auth-service:8000"
    methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    protocol: HTTP
    strip_prefix: false
    timeout: 120
    middlewares:
      require_auth: false
    rate_limit:
      requests: 100000
      period: "minute"
      circuit_breaker:
        enabled: true
        threshold: 5
        timeout: 30
        max_concurrent: 100
      retry_policy:
        enabled: true
        attempts: 3
        per_try_timeout: 5
        retry_on: ["connection_error", "server_error"]
```
2. Start the gateway:
```bash
docker-compose up -d
```
3. Make a request:
```bash
curl -H "x-api-key: your-api-key" http://localhost:8080/users
```

---

## ⚙️ Configuration

### Main Files
- `config.yaml`: Global gateway configuration
- `routes.yaml`: Route-specific configuration (see [Route Examples](#route-examples))

### Global Configuration (`config.yaml`)

| Section    | Key                      | Description                                 | Default                        |
|------------|--------------------------|---------------------------------------------|---------------------------------|
| **server** | `address`                | Server listening address                    | ":8080"                       |
|            | `read_timeout`           | Read timeout in seconds                     | 30                              |
|            | `write_timeout`          | Write timeout in seconds                    | 30                              |
|            | `idle_timeout`           | Idle connection timeout                     | 120                             |
|            | `max_header_bytes`       | Maximum header size                         | 1048576                         |
|            | `enable_http2`           | Enable HTTP/2 support                       | true                            |
|            | `enable_compression`     | Enable response compression                 | true                            |
| **auth**   | `jwt_secret`             | JWT signing secret                          | ${JWT_SECRET}                   |
|            | `jwt_expiry_hours`       | JWT token expiry in hours                   | 24                              |
|            | `api_key_validation_url` | API key validation endpoint                 | ${API_VALIDATION_URL}           |
|            | `api_key_header`         | API key header name                         | "x-api-key"                    |
| **logging**| `level`                  | Log level (debug, info, warn, error)        | info                            |
|            | `format`                 | Log format (json, console)                  | json                            |
|            | `enable_access_log`      | Enable access logging                       | true                            |
|            | `production_mode`        | Enable production logging                   | true                            |
|            | `stacktrace_level`       | Level for stacktrace capture                | error                           |
| **security**| `tls.enabled`           | Enable TLS                                  | false                           |
|            | `tls.cert_file`          | TLS certificate path                        | "/certs/server.crt"            |
|            | `tls.key_file`           | TLS key path                                | "/certs/server.key"            |
|            | `enable_xss_protection`  | Enable XSS protection                       | true                            |
|            | `enable_frame_deny`      | Enable clickjacking protection              | true                            |
|            | `max_body_size`          | Maximum request body size                   | 10485760                        |
| **cache**  | `enabled`                | Enable response caching                     | true                            |
|            | `default_ttl`            | Default cache TTL in seconds                | 60                              |
|            | `max_ttl`                | Maximum cache TTL                           | 3600                            |
|            | `max_size`               | Maximum cache entries                       | 1000                            |
| **tracing**| `enabled`                | Enable distributed tracing                  | true                            |
|            | `provider`               | Tracing provider (jaeger)                   | "jaeger"                       |
|            | `endpoint`               | Jaeger collector endpoint                   | http://jaeger:14268/api/traces  |
|            | `service_name`           | Service name in traces                      | "api-gateway"                  |
|            | `sample_rate`            | Trace sampling rate                         | 0.1                             |
| **metrics**| `enabled`                | Enable Prometheus metrics                   | true                            |
|            | `endpoint`               | Metrics endpoint                            | "/metrics"                     |
|            | `include_system`         | Include system metrics                      | true                            |

#### Environment Variables

| Variable              | Description                  | Default                                 |
|-----------------------|------------------------------|-----------------------------------------|
| `LOG_LEVEL`           | Logging level                | info                                    |
| `LOG_FORMAT`          | Log format                   | json                                    |
| `JWT_SECRET`          | JWT signing secret           | required                                |
| `API_VALIDATION_URL`  | API key validation URL       | required                                |
| `TRACING_ENDPOINT`    | Jaeger collector endpoint    | http://jaeger:14268/api/traces          |
| `ENV`                 | Environment name             | production                              |

### Route Configuration (`routes.yaml`)

| Section           | Key                        | Description                        | Example                        |
|-------------------|---------------------------|------------------------------------|--------------------------------|
| **Basic Route**   | `path`                    | Route path pattern                 | "/api/v1/users"               |
|                   | `methods`                 | Allowed HTTP methods               | ["GET", "POST"]               |
|                   | `upstream`                | Backend service URL                | "http://user-service:8080"    |
|                   | `protocol`                | Require Protocol                   | HTTP, SOCKET                   |
|                   | `strip_prefix`            | Remove path prefix                 | true                           |
| **Middlewares**   | `middlewares.require_auth` | Require authentication             | true                           |
|                   | `middlewares.rate_limit`   | Per-route rate limiting config     | see below                      |
|                   | `middlewares.cache`        | Per-route cache config             | see below                      |
|                   | `middlewares.circuit_breaker`| Per-route circuit breaker config | see below                      |
|                   | `middlewares.retry`        | Per-route retry config             | see below                      |
| **Load Balancing**| `method`                  | Load balancing algorithm           | "round_robin"                 |
|                   | `health_check`            | Enable health checks               | true                           |
|                   | `driver`                  | Where to obtain endpoints          | "static", "etcd"              |
|                   | `discoveries.name`        | service discovery name             | "myServers"                   |
|                   | `discoveries.prefix`      | service discovery prefix           | "services"                    |
|                   | `discoveries.fail_limit`  | Unable to obtain service address retry times | 3                  |
|                   | `endpoints`               | List of backend endpoints          | ["http://service:8080"]       |
|                   | `health_check_config.path`| Health check endpoint              | "/health"                     |
|                   | `health_check_config.interval`| Check interval in seconds       | 10                             |

#### Middleware Config Examples

- **Rate Limiting**
```yaml
middlewares:
  rate_limit:
    requests: 100
    period: "minute"
```

- **Caching**
```yaml
middlewares:
  cache:
    enabled: true
    ttl: 300
    cache_authenticated: false
```

- **Circuit Breaker**
```yaml
middlewares:
  circuit_breaker:
    enabled: true
    threshold: 5
    timeout: 30
    max_concurrent: 100
```

- **Retry Policy**
```yaml
middlewares:
  retry_policy:
    enabled: true
    attempts: 3
    per_try_timeout: 5
    retry_on: ["connection_error", "server_error"]
```

- **Authentication**
```yaml
middlewares:
  require_auth: true
```

### gRPC Configuration Options

- `protocol`: Set to "GRPC" for gRPC routes
- `endpoints_protocol`: Specifies the backend protocol ("GRPC" or "HTTP")
- `rpc_server`: Base path for the gRPC service
- `path`: Full gRPC service name pattern (e.g., "api_gateway.shop.user.v1.User/*")
- `upstream`: gRPC server address with "grpc://" scheme

### Middleware Support for gRPC

All standard middlewares work with gRPC:
- Authentication
- Rate limiting
- Circuit breaker
- Compression
- Timeout handling
- Error handling
- Header transformation
- Metrics collection
- Tracing

---

## 🛣️ Route Examples

### Basic Proxy
```yaml
routes:
  - path: "/api/v1/users"
    upstream: "http://user-service:8080"
    protocol: HTTP
    strip_prefix: false
    timeout: 120
```

### With Authentication
```yaml
routes:
  - path: "/scanjobmanager/*"
    upstream: "http://scanjobmanager:8001"
    protocol: HTTP
    strip_prefix: false
    timeout: 120
    middlewares:
      require_auth: true
```

### With Rate Limiting
```yaml
routes:
  - path: "/api/v1/search"
    upstream: "http://search-service:8080"
    protocol: HTTP
    middlewares:
      rate_limit:
        requests: 100000
        period: "minute"
```

### With Circuit Breaker
```yaml
routes:
  - path: "/api/v1/orders"
    upstream: "http://order-service:8080"
    protocol: HTTP
    middlewares:
      circuit_breaker:
        enabled: true
        threshold: 5
        timeout: 30
        max_concurrent: 100
```

### With Retry Policy
```yaml
routes:
  - path: "/api/v1/products"
    upstream: "http://product-service:8080"
    protocol: HTTP
    middlewares:
      retry_policy:
        enabled: true
        attempts: 3
        per_try_timeout: 5
        retry_on: ["connection_error", "server_error"]
```

### With Caching
```yaml
routes:
  - path: "/api/v1/cache"
    upstream: "http://cache-service:8080"
    protocol: HTTP
    middlewares:
      cache:
        enabled: true
        ttl: 300
        cache_authenticated: false
```

### Complete Example with All Middlewares
```yaml
routes:
  - path: "/project"
    upstream: "http://host.docker.internal:8002"
    methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    protocol: HTTP
    strip_prefix: false
    timeout: 120
    middlewares:
      require_auth: true
      rate_limit:
        requests: 100000
        period: "minute"
      circuit_breaker:
        enabled: true
        threshold: 5
        timeout: 30
        max_concurrent: 100
      retry_policy:
        enabled: true
        attempts: 3
        per_try_timeout: 5
        retry_on: ["connection_error", "server_error"]
```

### WebSocket Support
```yaml
routes:
  - path: "/ws"
    upstream: "ws://websocket-service:8080"
    protocol: SOCKET
    websocket:
      enabled: true
    middlewares:
      require_auth: true
```

### HTTP to HTTP Route
```yaml
routes:
  - path: "/api/users"
    protocol: "HTTP"
    upstream: "http://users-service:8080"
    methods: ["GET", "POST", "PUT", "DELETE"]
    strip_prefix: true
    timeout: 30
    compression: true
    middlewares:
      require_auth: true
      rate_limit:
        requests: 100000
        period: "minute"
```

### gRPC to gRPC Route
```yaml
routes:
  - path: "api_gateway.shop.user.v1.User/*"
    protocol: "GRPC"
    endpoints_protocol: "GRPC"
    rpc_server: "/api/user"
    upstream: "grpc://user-service:50051"
    timeout: 30
    compression: true
    middlewares:
      require_auth: true
      circuit_breaker:
        enabled: true
        threshold: 5
        timeout: 30
        max_concurrent: 100
```

### HTTP to gRPC Route (Protocol Conversion)
```yaml
routes:
  - path: "/api/products"
    protocol: "HTTP"
    endpoints_protocol: "GRPC"
    rpc_server: "/api/product"
    upstream: "grpc://product-service:50051"
    methods: ["GET", "POST"]
    timeout: 30
    middlewares:
      require_auth: true
```

### Service Discovery Example
```yaml
routes:
  - path: "/api/recommendations/*"
    methods: ["GET", "POST"]
    protocol: HTTP
    upstream: "http://recommendation-service:8090"
    strip_prefix: true
    timeout: 30
    load_balancing:
      method: "round_robin"
      health_check: true
      driver: etcd
      discoveries:
        name: "recommendation-service"
        prefix: "services"
        fail_limit: 3
      health_check_config:
        path: "/health"
        interval: 10
        timeout: 2
    middlewares:
      require_auth: true
      circuit_breaker:
        enabled: true
        threshold: 5
        timeout: 30
        max_concurrent: 100
```

---

## 🔒 Authentication

- **API Key**: `x-api-key` header or `api_key` query param
- **JWT**: `Authorization: Bearer ...` header or `token` query param
- Both can be required per route via `middlewares.require_auth`

**Examples:**
```bash
curl -H "x-api-key: your-api-key" http://localhost:8080/api/v1/users
curl -H "Authorization: Bearer your.jwt.token" http://localhost:8080/api/v1/users
curl "http://localhost:8080/api/v1/users?token=your.jwt.token"
curl "http://localhost:8080/api/v1/users?api_key=your-api-key"
```

---

## 🚦 Traffic Management

- **Rate Limiting**: Per route, per client, configurable period and burst
- **Circuit Breaker**: Per route, configurable thresholds and timeouts
- **Caching**: In-memory, per route, configurable TTL and vary headers
- **Retries**: Per route, with backoff
- **Load Balancing**: Static endpoints or service discovery

---

## 📊 Observability

- **Metrics**: Prometheus metrics at `/metrics`
- **Tracing**: Distributed tracing (Jaeger, OpenTelemetry)
- **Logging**: Structured JSON logs
- **Health Checks**: `/health` endpoint

---

## 🌍 Client IP & Geolocation

- **Client IP Detection**: Automatically extracts real client IP from headers (X-Real-IP, X-Forwarded-For, etc.)
- **Optional Geolocation**: If an IP2Location LITE database is present, country code is included in `/test-ip` and logs. If not, geolocation is disabled gracefully.

**To enable geolocation:**
1. Download the IP2Location LITE DB1 from [IP2Location](https://lite.ip2location.com/)
2. Place it in one of:
   - `./IP2LOCATION-LITE-DB1.BIN`
   - `./configs/IP2LOCATION-LITE-DB1.BIN`
   - `/etc/api-gateway/IP2LOCATION-LITE-DB1.BIN`
   - `/usr/share/ip2location/IP2LOCATION-LITE-DB1.BIN`
   - Or set the `IP2LOCATION_DB_PATH` env var

**Test endpoint:**
```bash
curl http://localhost:8080/test-ip | jq
curl -H "X-Real-IP: 8.8.8.8" http://localhost:8080/test-ip | jq
```

---

## 📚 API Documentation

- **Dynamic OpenAPI/Swagger**: Documentation is auto-generated from your `routes.yaml` and available at `/docs/swagger/`.
- **Includes**: All routes, methods, security schemes, path params, and more.
- **Updates**: On server start and whenever routes are changed.

**Example:**
```yaml
openapi: 3.0.3
info:
  title: Oortfy API Gateway
  description: API Gateway for Oortfy microservices
  version: 1.0.0
paths:
  /users:
    get:
      summary: Proxy to user-service
      security:
        - BearerAuth: []
        - ApiKeyAuth: []
      responses:
        '200':
          description: Success
```

---

## 🛠️ Development

### Building
```bash
make build
```

### Testing
```bash
make test
```

### Local Development
```bash
make dev
```

### Docker Support
```bash
docker-compose up
```

---

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

All contributions must be made back to this project as per our license terms.

---

## 📄 License

This project is licensed under the Mozilla Public License 2.0 with Commons Clause - see the [LICENSE](LICENSE) file for details.

Key points:
- ✅ You can use this software commercially
- ✅ You can modify the code
- ✅ You must share modifications back to this project
- ❌ You cannot sell this software as a standalone product
- ❌ You cannot distribute closed source versions
