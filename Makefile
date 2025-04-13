.PHONY: build run test clean docker-build docker-run docker-compose test-unit test-integration test-coverage test-race test-all swagger-validate swagger-serve

# Variables
APP_NAME=apigateway
MAIN_PATH=cmd/api/main.go
BUILD_DIR=bin
CONFIG_PATH=configs/config.yaml
ROUTES_PATH=configs/routes.yaml
COVERAGE_FILE=coverage.out
COVERAGE_HTML=coverage.html
SWAGGER_DIR=docs/swagger

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

test-unit:
	@echo "Running unit tests..."
	@go test -v `go list ./... | grep -v integration`

test-integration:
	@echo "Running integration tests..."
	@go test -v -tags=integration ./tests/...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=$(COVERAGE_FILE) ./...
	@go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"

test-race:
	@echo "Running tests with race detection..."
	@go test -race -v ./...

test-all: test-unit test-integration test-coverage test-race
	@echo "All tests completed successfully"

clean:
	@echo "Cleaning build directory..."
	@rm -rf $(BUILD_DIR) $(COVERAGE_FILE) $(COVERAGE_HTML)
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
		-e API_VALIDATION_URL=http://auth-service:8081/auth/validate-api-key \
		-e CONFIG_PATH=/app/configs/config.yaml \
		-e ROUTES_PATH=/app/configs/routes.yaml \
		-v $(PWD)/configs:/app/configs \
		$(APP_NAME)

docker-compose:
	@echo "Running with Docker Compose..."
	@docker-compose up -d

docker-test:
	@echo "Running tests in Docker..."
	@docker run --rm -v $(PWD):/app -w /app golang:1.20 go test -v ./...

# Mock services
mock-build:
	@echo "Building mock services..."
	@cd tests/mock_service && go build -o ../../$(BUILD_DIR)/mock-service

mock-run: mock-build
	@echo "Running mock service..."
	@SERVICE_NAME=mock-service PORT=8081 $(BUILD_DIR)/mock-service

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

# Swagger commands
swagger-validate:
	@echo "Validating Swagger file..."
	@chmod +x $(SWAGGER_DIR)/validate.sh
	@cd $(SWAGGER_DIR) && ./validate.sh

swagger-serve:
	@echo "Serving Swagger UI on http://localhost:8090/swagger/"
	@cd $(SWAGGER_DIR) && python3 -m http.server 8090

help:
	@echo "Available commands:"
	@echo "  make build         - Build the application"
	@echo "  make run           - Build and run the application"
	@echo "  make test          - Run all tests"
	@echo "  make test-unit     - Run unit tests only"
	@echo "  make test-integration - Run integration tests only"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make test-race     - Run tests with race detection"
	@echo "  make test-all      - Run all test suites"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-run    - Run Docker container with mounted config"
	@echo "  make docker-compose - Run with Docker Compose"
	@echo "  make docker-test   - Run tests in Docker container"
	@echo "  make mock-build    - Build mock services"
	@echo "  make mock-run      - Run a mock service locally"
	@echo "  make fmt           - Format code"
	@echo "  make vet           - Vet code"
	@echo "  make lint          - Lint code (requires golangci-lint)"
	@echo "  make deps          - Download dependencies"
	@echo "  make swagger-validate - Validate the Swagger documentation"
	@echo "  make swagger-serve - Serve the Swagger UI on http://localhost:8090/swagger/" 