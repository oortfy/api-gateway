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

- **Request Processing**
  - 🔄 URL Rewriting
  - 🔄 Header Transformation
  - 🔄 Query Parameter Manipulation
  - 🔄 WebSocket Support

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
  - path: "/users"
    methods: ["GET", "POST"]
    upstream: "http://user-service:8080"
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

### Main Configuration (config.yaml)

```yaml
server:
  address: ":8080"
  read_timeout: 30
  write_timeout: 30
  idle_timeout: 120

logging:
  level: "${LOG_LEVEL:-info}"
  format: "${LOG_FORMAT:-json}"
  output: "stdout"
  enable_access_log: true
  production_mode: true
  stacktrace_level: "error"
  sampling:
    enabled: true
    initial: 100
    thereafter: 100

security:
  tls:
    enabled: false
    cert_file: "/certs/server.crt"
    key_file: "/certs/server.key"
  enable_xss_protection: true
  enable_frame_deny: true
```

### Route Configuration (routes.yaml)

```yaml
routes:
  - path: "/users"
    methods: ["GET", "POST", "PUT", "DELETE"]
    upstream: "http://user-service:8080"
    strip_prefix: true
    require_auth: true
    timeout: 30
    
    rate_limit:
      requests: 100
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
      retry_on: ["503", "connect-failure"]
    
    cache:
      enabled: true
      ttl: 300
      vary_by_headers: ["Accept", "Accept-Encoding"]
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | info |
| `LOG_FORMAT` | Log format (json, console) | json |
| `JWT_SECRET` | Secret for JWT validation | - |
| `API_VALIDATION_URL` | URL for API key validation | - |

## 🛣️ Route Examples

### Basic Proxy
```yaml
routes:
  - path: "/api/v1/users"
    upstream: "http://user-service:8080"
```

### With Authentication
```yaml
routes:
  - path: "/api/v1/orders"
    upstream: "http://order-service:8080"
    require_auth: true
    auth_type: "jwt"
```

### With Rate Limiting
```yaml
routes:
  - path: "/api/v1/search"
    upstream: "http://search-service:8080"
    rate_limit:
      requests: 100
      period: "minute"
```

### WebSocket Support
```yaml
routes:
  - path: "/ws"
    upstream: "ws://websocket-service:8080"
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