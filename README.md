# API Gateway

A high-performance, feature-rich API Gateway built in Go, designed for microservices architectures with advanced traffic management, security, and observability features.

[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?style=flat&logo=go)](https://golang.org/doc/devel/release.html)
[![License](https://img.shields.io/badge/License-MPL%202.0-blue.svg)](LICENSE)

## 🚀 Features

- **Traffic Management**
  - ✅ Rate Limiting with client identification
  - ✅ Circuit Breaker protection
  - ✅ Request Retries with backoff
  - ✅ Load Balancing
  - ✅ Response Caching

- **Security**
  - 🔒 API Key Authentication
  - 🔒 JWT Token Validation
  - 🔒 CORS Configuration
  - 🔒 TLS Support
  - 🔒 Header Security (HSTS, XSS Protection)

- **Observability**
  - 📊 Prometheus Metrics
  - 🔍 Distributed Tracing (Jaeger)
  - 📝 Structured JSON Logging
  - 🏥 Health Checks
  - 🌍 Optional IP Geolocation

- **Request Processing**
  - 🔄 URL Rewriting
  - 🔄 Header Transformation
  - 🔄 Query Parameter Manipulation
  - 🔄 WebSocket Support

- **Documentation**
  - 📚 Dynamic OpenAPI/Swagger Documentation
  - 🔄 Auto-generated from routes configuration
  - 🔒 Security scheme documentation
  - 🔄 Real-time updates

## 📋 Table of Contents

- [Quick Start](#quick-start)
- [Why This API Gateway?](#why-this-api-gateway)
- [Configuration](#configuration)
- [Route Examples](#route-examples)
- [Authentication](#authentication)
- [Traffic Management](#traffic-management)
- [Observability](#observability)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)
- [Client IP Forwarding and Geolocation](#client-ip-forwarding-and-geolocation)
- [API Documentation](#api-documentation)

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
  - path: "/users/*"
    methods: ["GET", "POST"]
    upstream: "http://user-service:8080"
    protocol: HTTP
    strip_prefix: true
    require_auth: true
    rate_limit:
      requests: 100
      period: "minute"
  - path: "/manager/user"
    methods: ["GET", "POST"]
    upstream: "http://user-service:8080"
    protocol: HTTP
    strip_prefix: true
    require_auth: true
    rate_limit:
      requests: 100
      period: "minute"
```

2. Start the gateway:
```bash
docker-compose up -d
```

3. Make a request:
```bash
curl -H "x-api-key: your-api-key" http://localhost:8080/users
```

## 🤔 Why This API Gateway?

- **Simplicity**: Easy to configure and deploy
- **Performance**: Built in Go for high throughput
- **Security**: Built-in authentication and security features
- **Observability**: Complete monitoring and tracing
- **Flexibility**: Extensive configuration options
- **Reliability**: Circuit breakers and retries included

## ⚙️ Configuration

### Configuration Files Overview

The API Gateway uses two main configuration files:
- `config.yaml`: Global gateway configuration
- `routes.yaml`: Route-specific configuration

### Global Configuration (config.yaml)

| Section | Key | Description | Default |
|---------|-----|-------------|---------|
| **server** |
| | `address` | Server listening address | ":8080" |
| | `read_timeout` | Read timeout in seconds | 30 |
| | `write_timeout` | Write timeout in seconds | 30 |
| | `idle_timeout` | Idle connection timeout | 120 |
| | `max_header_bytes` | Maximum header size | 1048576 |
| | `enable_http2` | Enable HTTP/2 support | true |
| | `enable_compression` | Enable response compression | true |
| **auth** |
| | `jwt_secret` | JWT signing secret | ${JWT_SECRET} |
| | `jwt_expiry_hours` | JWT token expiry in hours | 24 |
| | `api_key_validation_url` | API key validation endpoint | ${API_VALIDATION_URL} |
| | `api_key_header` | API key header name | "x-api-key" |
| **logging** |
| | `level` | Log level (debug, info, warn, error) | info |
| | `format` | Log format (json, console) | json |
| | `enable_access_log` | Enable access logging | true |
| | `production_mode` | Enable production logging | true |
| | `stacktrace_level` | Level for stacktrace capture | error |
| **security** |
| | `tls.enabled` | Enable TLS | false |
| | `tls.cert_file` | TLS certificate path | "/certs/server.crt" |
| | `tls.key_file` | TLS key path | "/certs/server.key" |
| | `enable_xss_protection` | Enable XSS protection | true |
| | `enable_frame_deny` | Enable clickjacking protection | true |
| | `max_body_size` | Maximum request body size | 10485760 |
| **cache** |
| | `enabled` | Enable response caching | true |
| | `default_ttl` | Default cache TTL in seconds | 60 |
| | `max_ttl` | Maximum cache TTL | 3600 |
| | `max_size` | Maximum cache entries | 1000 |
| **tracing** |
| | `enabled` | Enable distributed tracing | true |
| | `provider` | Tracing provider (jaeger) | "jaeger" |
| | `endpoint` | Jaeger collector endpoint | http://jaeger:14268/api/traces |
| | `service_name` | Service name in traces | "api-gateway" |
| | `sample_rate` | Trace sampling rate | 0.1 |
| **metrics** |
| | `enabled` | Enable Prometheus metrics | true |
| | `endpoint` | Metrics endpoint | "/metrics" |
| | `include_system` | Include system metrics | true |

### Route Configuration (routes.yaml)

| Section | Key | Description                  | Example                    |
|---------|-----|------------------------------|----------------------------|
| **Basic Route** |
| | `path` | Route path pattern           | "/api/v1/users"            |
| | `methods` | Allowed HTTP methods         | ["GET", "POST"]            |
| | `upstream` | Backend service URL          | "http://user-service:8080" |
| | `protocol` | Require Protocol             | HTTP,SOCKET                      |
| | `strip_prefix` | Remove path prefix           | true                       |
| | `require_auth` | Require authentication       | true                       |
| **Load Balancing** |
| | `method` | Load balancing algorithm     | "round_robin"              |
| | `health_check` | Enable health checks         | true                       |
| | `endpoints` | List of backend endpoints    | ["http://service:8080"]    |
| | `health_check_config.path` | Health check endpoint        | "/health"                  |
| | `health_check_config.interval` | Check interval in seconds    | 10                         |
| **Rate Limiting** |
| | `requests_per_minute` | Request limit per minute     | 1000                       |
| | `burst` | Burst size for rate limiting | 50                         |
| **Circuit Breaker** |
| | `enabled` | Enable circuit breaker       | true                       |
| | `threshold` | Error threshold count        | 10                         |
| | `timeout` | Reset timeout in seconds     | 30                         |
| | `max_concurrent` | Max concurrent requests      | 3                          |
| **Retry Policy** |
| | `max_attempts` | Maximum retry attempts       | 3                          |
| | `initial_interval` | Initial retry interval       | 1                          |
| | `max_interval` | Maximum retry interval       | 5                          |
| | `multiplier` | Backoff multiplier           | 2.0                        |
| | `retry_on_status_codes` | Status codes to retry        | [500, 502, 503, 504]       |
| **Caching** |
| | `enabled` | Enable route caching         | true                       |
| | `ttl` | Cache TTL in seconds         | 300                        |
| | `vary_by_headers` | Headers affecting cache      | ["Accept"]                 |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Logging level | info |
| `LOG_FORMAT` | Log format | json |
| `JWT_SECRET` | JWT signing secret | required |
| `API_VALIDATION_URL` | API key validation URL | required |
| `TRACING_ENDPOINT` | Jaeger collector endpoint | http://jaeger:14268/api/traces |
| `ENV` | Environment name | production |

## 🛣️ Route Examples

### Basic Proxy
```yaml
routes:
  - path: "/api/v1/users"
    upstream: "http://user-service:8080"
    protocol: HTTP
```

### With Authentication
```yaml
routes:
  - path: "/api/v1/orders"
    upstream: "http://order-service:8080"
    protocol: HTTP
    require_auth: true
    auth_type: "jwt"
```

### With Rate Limiting
```yaml
routes:
  - path: "/api/v1/search"
    upstream: "http://search-service:8080"
    protocol: HTTP
    rate_limit:
      requests: 100
      period: "minute"
```

### WebSocket Support
```yaml
routes:
  - path: "/ws"
    upstream: "ws://websocket-service:8080"
    protocol: SOCKET
    websocket: true
```

## 🔒 Authentication

### API Key Authentication
```bash
curl -H "x-api-key: your-api-key" http://localhost:8080/api/v1/users
```

### JWT Authentication
```bash
curl -H "Authorization: Bearer your.jwt.token" http://localhost:8080/api/v1/users
```

### Query Parameter Authentication

For clients that cannot set custom headers, both API key and JWT token authentication can be provided via URL query parameters:

```bash
# JWT Authentication via query parameter
curl "http://localhost:8080/api/v1/users?token=your.jwt.token"

# API Key Authentication via query parameter
curl "http://localhost:8080/api/v1/users?api_key=your-api-key"
```

### WebSocket Authentication

WebSocket connections can be authenticated using the same methods as HTTP requests:

```javascript
// WebSocket with JWT in header (preferred in browser environments)
const socket = new WebSocket('ws://localhost:8080/ws');
socket.setRequestHeader('Authorization', 'Bearer your.jwt.token');

// WebSocket with token in URL (for environments that don't support custom headers)
const socket = new WebSocket('ws://localhost:8080/ws?token=your.jwt.token');
```

Secure WebSocket connections (wss://) are also supported and recommended for production environments.

## 🚦 Traffic Management

### Rate Limiting
- Client identification by IP, API key, or custom header
- Configurable limits and time windows
- Redis support for distributed rate limiting

### Circuit Breaker
- Protects downstream services
- Configurable thresholds and timeouts
- Automatic recovery with half-open state

### Caching
- In-memory caching with TTL
- Cache key generation based on headers
- Cache invalidation endpoints

## 📊 Observability

### Metrics
Access Prometheus metrics at `/metrics`:
```bash
curl http://localhost:8080/metrics
```

### Tracing
View traces in Jaeger UI at `http://localhost:16686`

### Logging
```json
{
  "level": "info",
  "timestamp": "2024-04-11T10:30:45.123Z",
  "service": "api-gateway",
  "event": "request_completed",
  "method": "GET",
  "path": "/api/v1/users",
  "status": 200,
  "duration_ms": 45
}
```

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

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

All contributions must be made back to this project as per our license terms.

## 📄 License

This project is licensed under the Mozilla Public License 2.0 with Commons Clause - see the [LICENSE](LICENSE) file for details.

Key points:
- ✅ You can use this software commercially
- ✅ You can modify the code
- ✅ You must share modifications back to this project
- ❌ You cannot sell this software as a standalone product
- ❌ You cannot distribute closed source versions

## Client IP Forwarding and Geolocation

The API Gateway provides advanced client IP detection and optional geolocation features.

### Client IP Detection

The gateway automatically detects client IPs from various headers in order of priority:
1. X-Real-IP
2. X-Forwarded-For
3. CF-Connecting-IP (Cloudflare)
4. True-Client-IP
5. Forwarded (RFC 7239)
6. RemoteAddr

### Optional IP Geolocation

IP geolocation is an optional feature that can be enabled by providing an IP2Location database file. The feature is gracefully disabled if the database is not available.

#### Enabling IP Geolocation

1. Download the IP2Location LITE database (DB1) from [IP2Location](https://lite.ip2location.com/)
2. Place the database file in one of these locations:
   - `./IP2LOCATION-LITE-DB1.BIN`
   - `./configs/IP2LOCATION-LITE-DB1.BIN`
   - `/etc/api-gateway/IP2LOCATION-LITE-DB1.BIN`
   - `/usr/share/ip2location/IP2LOCATION-LITE-DB1.BIN`
   - Or set the `IP2LOCATION_DB_PATH` environment variable

The gateway will automatically detect and use the database if available. If the database is not found, the geolocation feature will be disabled without errors.

## API Documentation

The API Gateway automatically generates OpenAPI/Swagger documentation based on your route configuration.

### Accessing the Documentation

The Swagger UI is available at `/docs/swagger/` and includes:
- All configured routes and their methods
- Authentication requirements
- Security schemes (JWT, API Key)
- Path parameters and wildcards

### Dynamic Updates

The documentation is automatically generated when:
1. The server starts
2. Routes are updated in `routes.yaml`

### Security Schemes

The documentation includes all supported authentication methods:
- JWT Bearer token
- API Key in header
- JWT token in query parameter
- API Key in query parameter

### Example Documentation

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