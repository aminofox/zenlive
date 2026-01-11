.PHONY: help build test clean fmt vet run-example coverage

# Default target
help:
	@echo "ZenLive SDK - Makefile Commands"
	@echo "================================"
	@echo "make build        - Build all packages"
	@echo "make test         - Run tests"
	@echo "make coverage     - Run tests with coverage"
	@echo "make fmt          - Format code with gofmt"
	@echo "make vet          - Run go vet"
	@echo "make clean        - Clean build artifacts"
	@echo "make run-example  - Run basic example"
	@echo "make all          - Format, vet, test, and build"

# Build all packages
build:
	@echo "Building ZenLive SDK..."
	@go build ./...
	@echo "Build complete!"

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	@gofmt -w .
	@echo "Code formatted!"

# Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@go clean
	@rm -f coverage.out coverage.html
	@rm -rf bin/
	@echo "Clean complete!"

# Run basic example
run-example:
	@echo "Running basic example..."
	@go run examples/basic/main.go

# Run all checks
all: fmt vet test build
	@echo "All checks passed!"

# Build example binaries
build-examples:
	@echo "Building examples..."
	@mkdir -p bin
	@go build -o bin/basic ./examples/basic
	@echo "Examples built in bin/"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies installed!"
