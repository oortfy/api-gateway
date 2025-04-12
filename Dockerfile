# Use multi-stage build for smaller final image
FROM golang:1.19-alpine AS builder

# Set necessary environment variables
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

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

# Create app directory with proper permissions
RUN mkdir -p /app/configs

# Download IP2Location Lite database
RUN apk add --no-cache curl unzip && \
    curl -L "https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.BIN.ZIP" -o ip2location.zip && \
    unzip ip2location.zip -d /app/configs && \
    rm ip2location.zip && \
    apk del curl unzip

# Copy executable from builder stage
COPY --from=builder /app/apigateway /app/apigateway

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set ownership of the application directory
RUN chown -R appuser:appgroup /app

# Set working directory
WORKDIR /app

# Use the non-root user
USER appuser

# Set default environment variables that can be overridden at runtime
ENV CONFIG_PATH=/app/configs/config.yaml \
    ROUTES_PATH=/app/configs/routes.yaml \
    IP2LOCATION_DB_PATH="/app/configs/IP2LOCATION-LITE-DB1.BIN"

# Command to run
ENTRYPOINT ["/app/apigateway"] 