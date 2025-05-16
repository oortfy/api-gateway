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
  - Response caching (configurable per route)
  - Load balancing (static, service discovery via etcd)

- **Security**
  - API Key and JWT authentication (header or query param)
  - CORS configuration

- **Observability**
  - Prometheus metrics
  - Structured JSON logging
  - Health checks

- **Protocol Support**
  - **HTTP Proxying**: Traditional HTTP/HTTPS reverse proxy
  - **gRPC Support**: Basic gRPC routing capabilities
    - gRPC server implementation
    - Connection pooling
    - Unary method support

## 📋 Table of Contents
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Route Examples](#route-examples)
- [Authentication](#authentication)
- [gRPC Support](#grpc-support)
- [Development](#development)
- [License](#license)

---

## 🚀 Quick Start

### Prerequisites
- Go 1.20+
- Docker & Docker Compose

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
  - path: "/api/users/*"
    upstream: "http://users-service:8000"
    methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    protocol: HTTP
    strip_prefix: true
    timeout: 30
    middlewares:
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
curl -H "x-api-key: your-api-key" http://localhost:8080/api/users
```

---

## ⚙️ Configuration

### Main Files
- `config.yaml`: Global gateway configuration
- `routes.yaml`: Route-specific configuration

### Example Route Configurations

#### Basic HTTP Proxy
```yaml
routes:
  - path: "/api/users"
    upstream: "http://user-service:8080"
    protocol: HTTP
    strip_prefix: false
    timeout: 30
```

#### With Authentication
```yaml
routes:
  - path: "/api/products/*"
    upstream: "http://product-service:8001"
    protocol: HTTP
    strip_prefix: true
    timeout: 30
    middlewares:
      require_auth: true
```

#### With Rate Limiting
```yaml
routes:
  - path: "/api/search"
    upstream: "http://search-service:8080"
    protocol: HTTP
    middlewares:
      rate_limit:
        requests: 100
        period: "minute"
```

#### With Circuit Breaker
```yaml
routes:
  - path: "/api/orders"
    upstream: "http://order-service:8080"
    protocol: HTTP
    middlewares:
      circuit_breaker:
        enabled: true
        threshold: 5
        timeout: 30
        max_concurrent: 100
```

#### With Caching
```yaml
routes:
  - path: "/api/products"
    upstream: "http://product-service:8080"
    protocol: HTTP
    middlewares:
      cache:
        enabled: true
        ttl: 300
        cache_authenticated: false
```

#### With Service Discovery
```yaml
routes:
  - path: "/api/services"
    upstream: "etcd://services/api"  # Uses etcd service discovery
    protocol: HTTP
    load_balancing:
      strategy: "round_robin"  # Supports round_robin or random
```

## 🔒 Authentication

The API Gateway supports two authentication methods:

- **API Key**: `x-api-key` header or `api_key` query param
- **JWT**: `Authorization: Bearer ...` header or `token` query param

Authentication can be required per route via `middlewares.require_auth: true`

**Examples:**
```bash
curl -H "x-api-key: your-api-key" http://localhost:8080/api/users
curl -H "Authorization: Bearer your.jwt.token" http://localhost:8080/api/users
curl "http://localhost:8080/api/users?token=your.jwt.token"
curl "http://localhost:8080/api/users?api_key=your-api-key"
```

## 📊 Observability

- **Metrics**: Prometheus metrics at `/metrics`
- **Logging**: Structured JSON logs
- **Health Checks**: `/health` endpoint

## gRPC Support

The API Gateway includes basic gRPC functionality:

### Features
- gRPC server implementation
- Connection pooling
- Support for unary methods

### Configuration Example

```yaml
routes:
  - path: "test.service.TestService/*"
    protocol: "GRPC"
    endpoints_protocol: "GRPC"
    rpc_server: "/api/test"
    upstream: "grpc://localhost:50051"
```

Note: gRPC streaming is not yet supported.

## 🛠️ Development

### Building
```bash
make build
```

### Testing
```bash
make test
```

### Docker Support
```bash
docker-compose up
```

## 📄 License

This project is licensed under the Mozilla Public License 2.0 with Commons Clause - see the [LICENSE](LICENSE) file for details.

Key points:
- ✅ You can use this software commercially
- ✅ You can modify the code
- ✅ You must share modifications back to this project
- ❌ You cannot sell this software as a standalone product
- ❌ You cannot distribute closed source versions
