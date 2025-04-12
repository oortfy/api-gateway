FROM golang:1.19-alpine AS builder

# Set necessary environment variables
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Create appuser
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -ldflags="-s -w" -o apigateway ./cmd/api

# Create a minimal image
FROM alpine:3.16

# Import from builder image
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /app/apigateway /app/apigateway

# Create directories for config
RUN mkdir -p /app/configs

# Use an unprivileged user
USER appuser

# Set working directory
WORKDIR /app

# Set default environment variables that can be overridden at runtime
ENV CONFIG_PATH=/app/configs/config.yaml \
    ROUTES_PATH=/app/configs/routes.yaml

# Download IP2Location Lite database
RUN apk add --no-cache curl unzip && \
    mkdir -p /app/configs && \
    # Download the free IP2Location LITE database
    curl -L "https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.BIN.ZIP" -o ip2location.zip && \
    unzip ip2location.zip -d /app/configs && \
    rm ip2location.zip && \
    apk del curl unzip

# Set environment variables
ENV IP2LOCATION_DB_PATH="/app/configs/IP2LOCATION-LITE-DB1.BIN"

# Command to run
ENTRYPOINT ["/app/apigateway"] 