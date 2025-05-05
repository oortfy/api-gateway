#!/bin/bash

# Simple script to validate the Swagger/OpenAPI file

echo "Validating Swagger file..."

# Check if swagger-cli is installed
if ! command -v swagger-cli &> /dev/null; then
    echo "swagger-cli is not installed. Installing..."
    npm install -g @apidevtools/swagger-cli
fi

# Validate the swagger.yaml file
swagger-cli validate ./swagger.yaml

if [ $? -eq 0 ]; then
    echo "Swagger file is valid!"
    exit 0
else
    echo "Swagger file validation failed!"
    exit 1
fi 