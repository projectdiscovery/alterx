.PHONY: all build test lint clean

# Default target
all: build

# Build the alterx binary
build:
	@echo "Building alterx..."
	@go build -o alterx cmd/alterx/main.go
	@echo "Build complete: ./alterx"

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run linters
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found, running go vet..."; \
		go vet ./...; \
	fi

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f alterx
	@echo "Clean complete"
