.PHONY: build run test clean docker-build docker-run docker-compose

# Variables
APP_NAME=apigateway
MAIN_PATH=cmd/api/main.go
BUILD_DIR=bin
CONFIG_PATH=configs/config.yaml
ROUTES_PATH=configs/routes.yaml

# Go commands
build:
	@echo "Building application..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

run: build
	@echo "Running application..."
	@CONFIG_PATH=$(CONFIG_PATH) ROUTES_PATH=$(ROUTES_PATH) $(BUILD_DIR)/$(APP_NAME)

test:
	@echo "Running tests..."
	@go test -v ./...

clean:
	@echo "Cleaning build directory..."
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Docker commands
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(APP_NAME) .
	@echo "Docker build complete"

docker-run:
	@echo "Running Docker container..."
	@docker run -p 8080:8080 \
		-e JWT_SECRET=your_jwt_secret_here \
		-e CONFIG_PATH=/app/configs/config.yaml \
		-e ROUTES_PATH=/app/configs/routes.yaml \
		-v $(PWD)/configs:/app/configs \
		$(APP_NAME)

docker-compose:
	@echo "Running with Docker Compose..."
	@docker-compose up -d

# Other helpful commands
fmt:
	@echo "Formatting code..."
	@go fmt ./...

vet:
	@echo "Vetting code..."
	@go vet ./...

lint:
	@echo "Linting code..."
	@golangci-lint run ./...

deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

help:
	@echo "Available commands:"
	@echo "  make build         - Build the application"
	@echo "  make run           - Build and run the application"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-run    - Run Docker container with mounted config"
	@echo "  make docker-compose - Run with Docker Compose"
	@echo "  make fmt           - Format code"
	@echo "  make vet           - Vet code"
	@echo "  make lint          - Lint code (requires golangci-lint)"
	@echo "  make deps          - Download dependencies" 