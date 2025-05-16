#!/bin/bash
set -e

echo "Installing required dependencies..."
go get -v github.com/golang-jwt/jwt/v4
go get -v gopkg.in/yaml.v3
go get -v github.com/ip2location/ip2location-go/v9
go get -v github.com/stretchr/testify

echo "Running go build with buildvcs=false..."
go build -buildvcs=false -o /tmp/apigateway ./cmd/api

echo "Running tests for specific packages..."
go test -v -buildvcs=false ./internal/config/... ./internal/middleware/... ./internal/handlers/...

echo "All tests completed!" 