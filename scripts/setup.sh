#!/bin/bash

# Margin Offer System Setup Script

set -e

echo "Setting up Margin Offer System development environment..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
REQUIRED_VERSION="1.21"

if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
    echo "Go version $GO_VERSION is too old. Please upgrade to Go 1.21 or later."
    exit 1
fi

echo "✓ Go version $GO_VERSION detected"

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "Installing protoc..."
    
    # Detect OS
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        # Linux
        sudo apt-get update
        sudo apt-get install -y protobuf-compiler
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        if command -v brew &> /dev/null; then
            brew install protobuf
        else
            echo "Please install Homebrew first, then run: brew install protobuf"
            exit 1
        fi
    else
        echo "Please install protoc manually for your operating system"
        exit 1
    fi
else
    echo "✓ protoc detected"
fi

# Install Go dependencies
echo "Installing Go dependencies..."
go mod tidy

# Install protoc plugins
echo "Installing protoc plugins..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate protobuf files
echo "Generating protobuf files..."
make proto-gen

# Install development tools
echo "Installing development tools..."
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Check if Docker is installed
if command -v docker &> /dev/null; then
    echo "✓ Docker detected"
else
    echo "⚠️  Docker not found. Install Docker to use containerized services."
fi

# Check if docker-compose is available
if command -v docker-compose &> /dev/null || docker compose version &> /dev/null 2>&1; then
    echo "✓ Docker Compose detected"
else
    echo "⚠️  Docker Compose not found. Install Docker Compose to use multi-service setup."
fi

# Create necessary directories
echo "Creating directories..."
mkdir -p bin/
mkdir -p logs/
mkdir -p config/secrets/

# Build the project
echo "Building project..."
make build

echo ""
echo "🎉 Setup complete!"
echo ""
echo "Quick start:"
echo "  make run-store          # Start the store service"
echo "  make run-etl            # Start the ETL pipeline"
echo "  make run-backfiller     # Start the backfiller"
echo "  make dev-run-all        # Start all services"
echo ""
echo "Docker:"
echo "  docker-compose up       # Start all services with Docker"
echo ""
echo "Testing:"
echo "  make test               # Run tests"
echo "  make test-coverage      # Run tests with coverage"
echo ""
echo "Development:"
echo "  make lint               # Run linters"
echo "  make fmt                # Format code"
echo "  make help               # Show all available commands"
echo ""