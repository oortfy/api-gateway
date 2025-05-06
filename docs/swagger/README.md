# API Gateway Swagger Documentation

This directory contains the Swagger/OpenAPI documentation for the API Gateway.

## Overview

The Swagger documentation provides a comprehensive view of the API Gateway's features, endpoints, and authentication mechanisms. It serves as an interactive reference for developers working with the gateway.

## Accessing the Documentation

Once the API Gateway is running, you can access the Swagger UI at:

```
http://localhost:8080/docs/swagger/
```

This will display an interactive UI where you can:
- Explore available endpoints
- View request/response schemas
- Test API endpoints directly (with proper authentication)
- Understand authentication requirements

## Documentation Structure

The OpenAPI specification is defined in `swagger.yaml`, which documents:

1. **Gateway Features** - Overall capabilities and features
2. **Core Endpoints** - Built-in endpoints like health checks and metrics
3. **Authentication Mechanisms** - JWT, API key, and query parameter methods
4. **Generic Proxy Patterns** - How the gateway proxies requests
5. **WebSocket Support** - WebSocket connection handling

## Authentication in Swagger

The documentation describes four authentication methods:

1. **BearerAuth** - JWT token in the Authorization header
2. **ApiKeyAuth** - API key in the x-api-key header
3. **QueryTokenAuth** - JWT token in the 'token' query parameter
4. **QueryApiKeyAuth** - API key in the 'api_key' query parameter

These methods reflect the flexibility of the API Gateway's authentication system.

## Testing with Swagger UI

You can use the Swagger UI to test endpoints by:

1. Clicking on an endpoint to expand it
2. Clicking the "Try it out" button
3. Filling in required parameters
4. Providing authentication if needed
5. Clicking "Execute"

For `test-ip` or other built-in endpoints, this will work directly. For proxied endpoints, you'll need to have appropriate upstream services configured in the gateway.

## Customizing the Documentation

To customize the Swagger documentation:

1. Edit `swagger.yaml` to reflect your specific gateway configuration
2. Add details about your specific routes and authentication requirements
3. Restart the API Gateway to apply changes

## Exporting the Documentation

You can download the OpenAPI specification from the Swagger UI by clicking the "Download" button at the top of the page. This allows you to:

- Share the documentation with team members
- Import it into API tools like Postman
- Generate client libraries for various programming languages

## Further Resources

- [OpenAPI Specification](https://spec.openapis.org/oas/latest.html)
- [Swagger UI Documentation](https://swagger.io/tools/swagger-ui/)
- [API Gateway Documentation](../README.md) 