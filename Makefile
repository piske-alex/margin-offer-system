.PHONY: build test clean proto-gen run-store run-etl run-backfiller docker-build

# Variables
GO_VERSION := 1.21
APP_NAME := margin-offer-system
DOCKER_REGISTRY := your-registry.com
VERSION := $(shell git describe --tags --always --dirty)

# Build targets
build: proto-gen
	@echo "Building binaries..."
	go build -o bin/store cmd/store/main.go
	go build -o bin/realtime-etl cmd/realtime-etl/main.go
	go build -o bin/backfillers cmd/backfillers/main.go

# Test targets
test:
	@echo "Running tests..."
	go test -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Proto generation
proto-gen:
	@echo "Generating protobuf files..."
	mkdir -p proto/gen/go
	PATH=$$PATH:$$(go env GOPATH)/bin protoc --go_out=proto/gen/go --go_opt=paths=source_relative \
	       --go-grpc_out=proto/gen/go --go-grpc_opt=paths=source_relative \
	       proto/*.proto

# Run targets
run-store:
	@echo "Running store service..."
	go run cmd/store/main.go

run-etl:
	@echo "Running ETL pipeline..."
	go run cmd/realtime-etl/main.go

run-backfiller:
	@echo "Running backfiller..."
	go run cmd/backfillers/main.go --store-addr localhost:8080 --source chain

run-backfiller-once:
	@echo "Running one-time backfill..."
	go run cmd/backfillers/main.go -run-once -hours 24

run-marginfi:
	@echo "Running MarginFi sync service..."
	cd backfillers/marginfi && npm start

run-marginfi-dev:
	@echo "Running MarginFi sync service in development mode..."
	cd backfillers/marginfi && npm run dev

# Development targets
dev-setup:
	@echo "Setting up development environment..."
	go mod tidy
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Setting up MarginFi service dependencies..."
	cd backfillers/marginfi && npm install

dev-run-all:
	@echo "Starting all services in development mode..."
	@$(MAKE) run-store &
	@sleep 2
	@$(MAKE) run-etl &
	@$(MAKE) run-backfiller &
	@echo "All services started. Press Ctrl+C to stop."
	@wait

# Docker targets
docker-build:
	@echo "Building Docker images..."
	docker build -t $(DOCKER_REGISTRY)/$(APP_NAME)-store:$(VERSION) -f docker/Dockerfile.store .
	docker build -t $(DOCKER_REGISTRY)/$(APP_NAME)-etl:$(VERSION) -f docker/Dockerfile.etl .
	docker build -t $(DOCKER_REGISTRY)/$(APP_NAME)-backfiller:$(VERSION) -f docker/Dockerfile.backfiller .

docker-push:
	@echo "Pushing Docker images..."
	docker push $(DOCKER_REGISTRY)/$(APP_NAME)-store:$(VERSION)
	docker push $(DOCKER_REGISTRY)/$(APP_NAME)-etl:$(VERSION)
	docker push $(DOCKER_REGISTRY)/$(APP_NAME)-backfiller:$(VERSION)

docker-compose-up:
	@echo "Starting services with docker-compose..."
	docker-compose up -d

docker-compose-down:
	@echo "Stopping services with docker-compose..."
	docker-compose down

# Lint and format
lint:
	@echo "Running linters..."
	golangci-lint run

fmt:
	@echo "Formatting code..."
	go fmt ./...

# Clean targets
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf proto/gen/
	rm -f coverage.out coverage.html

# Database targets
db-migrate:
	@echo "Running database migrations..."
	# Add migration commands here

db-seed:
	@echo "Seeding database with test data..."
	# Add seed commands here

# Help target
help:
	@echo "Available targets:"
	@echo "  build              - Build all binaries"
	@echo "  test               - Run tests"
	@echo "  test-coverage      - Run tests with coverage"
	@echo "  proto-gen          - Generate protobuf files"
	@echo "  run-store          - Run store service"
	@echo "  run-etl            - Run ETL pipeline"
	@echo "  run-backfiller     - Run backfiller service"
	@echo "  run-backfiller-once - Run one-time backfill"
	@echo "  dev-setup          - Setup development environment"
	@echo "  dev-run-all        - Run all services in development"
	@echo "  docker-build       - Build Docker images"
	@echo "  docker-push        - Push Docker images"
	@echo "  lint               - Run linters"
	@echo "  fmt                - Format code"
	@echo "  clean              - Clean build artifacts"