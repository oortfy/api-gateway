#!/bin/bash

# Default values
CONFIG_PATH=${CONFIG_PATH:-"configs/config.yaml"}
ROUTES_PATH=${ROUTES_PATH:-"configs/routes.yaml"}
JWT_SECRET=${JWT_SECRET:-""}
API_VALIDATION_URL=${API_VALIDATION_URL:-""}
LOG_LEVEL=${LOG_LEVEL:-"info"}
LOG_FORMAT=${LOG_FORMAT:-"json"}

# Check for required environment variables
if [ -z "$JWT_SECRET" ]; then
    echo "ERROR: JWT_SECRET environment variable is required"
    exit 1
fi

if [ -z "$API_VALIDATION_URL" ]; then
    echo "ERROR: API_VALIDATION_URL environment variable is required"
    exit 1
fi

# Export all environment variables
export JWT_SECRET
export API_VALIDATION_URL
export LOG_LEVEL
export LOG_FORMAT

# Build the application if the binary doesn't exist
if [ ! -f "bin/apigateway" ]; then
    echo "Building the API Gateway..."
    make build
fi

# Run the application with specified config files
echo "Starting API Gateway..."
echo "Config file: $CONFIG_PATH"
echo "Routes file: $ROUTES_PATH"
echo "API validation URL: $API_VALIDATION_URL"
echo "Log level: $LOG_LEVEL"
echo "Log format: $LOG_FORMAT"

CONFIG_PATH="$CONFIG_PATH" ROUTES_PATH="$ROUTES_PATH" bin/apigateway 