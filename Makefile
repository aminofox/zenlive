.PHONY: help build test clean fmt vet run-example coverage server docker docker-push docker-pull

# Docker configuration
DOCKER_IMAGE ?= aminofox/zenlive
DOCKER_TAG ?= latest
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Default target
help:
	@echo "ZenLive SDK - Makefile Commands"
	@echo "================================"
	@echo "make build        - Build all packages"
	@echo "make server       - Build server binary"
	@echo "make test         - Run tests"
	@echo "make coverage     - Run tests with coverage"
	@echo "make fmt          - Format code with gofmt"
	@echo "make vet          - Run go vet"
	@echo "make clean        - Clean build artifacts"
	@echo "make run-example  - Run basic example"
	@echo "make all          - Format, vet, test, and build"
	@echo ""
	@echo "Docker Commands:"
	@echo "make docker       - Build Docker image"
	@echo "make docker-push  - Push image to Docker Hub"
	@echo "make docker-pull  - Pull image from Docker Hub"
	@echo "make docker-run   - Run server from Docker image"

# Build all packages
build:
	@echo "Building ZenLive SDK..."
	@go build ./...
	@echo "Build complete!"

# Build server binary
server:
	@echo "Building ZenLive server..."
	@mkdir -p bin
	@go build -o bin/zenlive-server ./cmd/zenlive-server
	@echo "Server binary created at: bin/zenlive-server"

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

# ============================================================================
# Docker targets
# ============================================================================

# Docker configuration
DOCKER_IMAGE ?= aminofox/zenlive
DOCKER_TAG ?= latest
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

docker:
	@echo "Building Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)"
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ') \
		.
	@docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):$(VERSION)
	@echo "Docker image built successfully!"
	@echo "  $(DOCKER_IMAGE):$(DOCKER_TAG)"
	@echo "  $(DOCKER_IMAGE):$(VERSION)"

docker-push:
	@echo "Pushing Docker images to Docker Hub..."
	@docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	@docker push $(DOCKER_IMAGE):$(VERSION)
	@echo "Images pushed successfully!"

docker-pull:
	@echo "Pulling Docker image from Docker Hub..."
	@docker pull $(DOCKER_IMAGE):$(DOCKER_TAG)
	@echo "Image pulled successfully!"

docker-run:
	@echo "Running ZenLive server from Docker image..."
	@docker run -d \
		--name zenlive-server \
		-p 7880:7880 \
		-p 7881:7881 \
		-p 9090:9090 \
		-e ZENLIVE_DEV_MODE=true \
		$(DOCKER_IMAGE):$(DOCKER_TAG)
	@echo "Server started! Check logs with: docker logs -f zenlive-server"
	@echo "Health check: curl http://localhost:7880/api/health"

docker-stop:
	@echo "Stopping ZenLive server..."
	@docker stop zenlive-server || true
	@docker rm zenlive-server || true
	@echo "Server stopped!"
