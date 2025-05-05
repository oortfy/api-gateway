#!/bin/bash

# Default values
CONFIG_PATH=${CONFIG_PATH:-"configs/config.yaml"}
ROUTES_PATH=${ROUTES_PATH:-"configs/routes.yaml"}
JWT_SECRET=${JWT_SECRET:-""}
API_VALIDATION_URL=${API_VALIDATION_URL:-""}
LOG_LEVEL=${LOG_LEVEL:-"info"}
LOG_FORMAT=${LOG_FORMAT:-"json"}
IP2LOCATION_DB_PATH=${IP2LOCATION_DB_PATH:-"configs/IP2LOCATION-LITE-DB1.BIN"}

# Check for required environment variables
if [ -z "$JWT_SECRET" ]; then
    echo "ERROR: JWT_SECRET environment variable is required"
    exit 1
fi

if [ -z "$API_VALIDATION_URL" ]; then
    echo "ERROR: API_VALIDATION_URL environment variable is required"
    exit 1
fi

# Check for IP2Location database and download it if not exists
if [ ! -f "$IP2LOCATION_DB_PATH" ]; then
    echo "IP2Location database not found at $IP2LOCATION_DB_PATH"
    echo "Downloading IP2Location LITE database..."
    
    # Create configs directory if it doesn't exist
    mkdir -p $(dirname "$IP2LOCATION_DB_PATH")
    
    # Download and extract the database
    curl -L "https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.BIN.ZIP" -o ip2location.zip
    unzip -o ip2location.zip -d $(dirname "$IP2LOCATION_DB_PATH")
    rm ip2location.zip
    
    if [ -f "$IP2LOCATION_DB_PATH" ]; then
        echo "IP2Location database downloaded successfully to $IP2LOCATION_DB_PATH"
    else
        echo "Failed to download IP2Location database"
    fi
fi

# Export all environment variables
export JWT_SECRET
export API_VALIDATION_URL
export LOG_LEVEL
export LOG_FORMAT
export IP2LOCATION_DB_PATH

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
echo "IP2Location database: $IP2LOCATION_DB_PATH"
echo "Log level: $LOG_LEVEL"
echo "Log format: $LOG_FORMAT"

CONFIG_PATH="$CONFIG_PATH" ROUTES_PATH="$ROUTES_PATH" bin/apigateway 