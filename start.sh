#!/bin/bash

# Default values
CONFIG_PATH=${CONFIG_PATH:-"configs/config.yaml"}
ROUTES_PATH=${ROUTES_PATH:-"configs/routes.yaml"}
JWT_SECRET=${JWT_SECRET:-"your_jwt_secret_here"}

# Export the JWT_SECRET
export JWT_SECRET

# Build the application if the binary doesn't exist
if [ ! -f "bin/apigateway" ]; then
    echo "Building the API Gateway..."
    make build
fi

# Run the application with specified config files
echo "Starting API Gateway..."
echo "Config file: $CONFIG_PATH"
echo "Routes file: $ROUTES_PATH"

CONFIG_PATH="$CONFIG_PATH" ROUTES_PATH="$ROUTES_PATH" bin/apigateway 