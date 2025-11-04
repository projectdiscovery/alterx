# Makefile for AlterX - Fast subdomain wordlist generator
# Project: github.com/projectdiscovery/alterx

# Build variables
BINARY_NAME := alterx
MAIN_PATH := ./cmd/alterx
PKG := github.com/projectdiscovery/alterx
GO := go
GOFLAGS := -trimpath
LDFLAGS := -s -w

# Version information (from git)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags with version info
BUILD_LDFLAGS := $(LDFLAGS) \
	-X '$(PKG)/internal/runner.Version=$(VERSION)' \
	-X '$(PKG)/internal/runner.Commit=$(COMMIT)' \
	-X '$(PKG)/internal/runner.BuildDate=$(BUILD_DATE)'

# Output directory
BUILD_DIR := bin
BINARY_PATH := $(BUILD_DIR)/$(BINARY_NAME)

# Color output
CYAN := \033[0;36m
RESET := \033[0m

.PHONY: all
all: help

.PHONY: help
help: ## Show this help message
	@echo "$(CYAN)AlterX - Makefile targets:$(RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-20s$(RESET) %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the binary
	@echo "$(CYAN)Building $(BINARY_NAME)...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(BUILD_LDFLAGS)" -o $(BINARY_PATH) $(MAIN_PATH)
	@echo "$(CYAN)✓ Binary built: $(BINARY_PATH)$(RESET)"

.PHONY: install
install: ## Install the binary to GOPATH/bin
	@echo "$(CYAN)Installing $(BINARY_NAME)...$(RESET)"
	CGO_ENABLED=0 $(GO) install $(GOFLAGS) -ldflags "$(BUILD_LDFLAGS)" $(MAIN_PATH)
	@echo "$(CYAN)✓ Installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)$(RESET)"

.PHONY: run
run: ## Run the application (use ARGS="..." to pass arguments)
	@echo "$(CYAN)Running $(BINARY_NAME)...$(RESET)"
	$(GO) run $(MAIN_PATH) $(ARGS)

.PHONY: test
test: ## Run all tests
	@echo "$(CYAN)Running tests...$(RESET)"
	$(GO) test ./... -v

.PHONY: test-short
test-short: ## Run tests without verbose output
	@echo "$(CYAN)Running tests...$(RESET)"
	$(GO) test ./...

.PHONY: test-race
test-race: ## Run tests with race detection
	@echo "$(CYAN)Running tests with race detector...$(RESET)"
	$(GO) test -race ./... -v

.PHONY: test-cover
test-cover: ## Run tests with coverage report
	@echo "$(CYAN)Running tests with coverage...$(RESET)"
	$(GO) test ./... -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(CYAN)✓ Coverage report: coverage.html$(RESET)"

.PHONY: test-inducer
test-inducer: ## Run tests for the inducer package
	@echo "$(CYAN)Running inducer tests...$(RESET)"
	@if $(GO) list ./internal/inducer/... ./inducer/... >/dev/null 2>&1; then \
		$(GO) test -v ./internal/inducer/... ./inducer/...; \
	else \
		echo "No inducer package found"; \
	fi

.PHONY: bench
bench: ## Run benchmarks
	@echo "$(CYAN)Running benchmarks...$(RESET)"
	$(GO) test -bench=. -benchmem ./...

.PHONY: lint
lint: ## Run golangci-lint
	@echo "$(CYAN)Running linter...$(RESET)"
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run --timeout=5m

.PHONY: fmt
fmt: ## Format code with gofmt
	@echo "$(CYAN)Formatting code...$(RESET)"
	$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo "$(CYAN)Running go vet...$(RESET)"
	$(GO) vet ./...

.PHONY: tidy
tidy: ## Tidy go modules
	@echo "$(CYAN)Tidying go modules...$(RESET)"
	$(GO) mod tidy

.PHONY: verify
verify: fmt vet lint test ## Run all verification steps (fmt, vet, lint, test)

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(CYAN)Cleaning build artifacts...$(RESET)"
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	$(GO) clean -cache -testcache
	@echo "$(CYAN)✓ Cleaned$(RESET)"

.PHONY: deps
deps: ## Download dependencies
	@echo "$(CYAN)Downloading dependencies...$(RESET)"
	$(GO) mod download

.PHONY: update-deps
update-deps: ## Update dependencies
	@echo "$(CYAN)Updating dependencies...$(RESET)"
	$(GO) get -u ./...
	$(GO) mod tidy

.PHONY: example
example: ## Run the example program
	@echo "$(CYAN)Running example...$(RESET)"
	$(GO) run examples/main.go

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "$(CYAN)Building Docker image...$(RESET)"
	docker build -t projectdiscovery/alterx:$(VERSION) .
	docker tag projectdiscovery/alterx:$(VERSION) projectdiscovery/alterx:latest
	@echo "$(CYAN)✓ Docker image built$(RESET)"

.PHONY: version
version: ## Show version information
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Go Version: $(shell go version)"

.PHONY: info
info: version ## Show project information
	@echo "Binary:     $(BINARY_NAME)"
	@echo "Package:    $(PKG)"
	@echo "Main Path:  $(MAIN_PATH)"
	@echo "Build Dir:  $(BUILD_DIR)"
