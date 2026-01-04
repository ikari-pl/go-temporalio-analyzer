# temporal-analyzer Makefile
# Build and install the Temporal workflow analyzer

# Binary name
BINARY_NAME := temporal-analyzer

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOVET := $(GOCMD) vet
GOFMT := gofmt

# Build flags
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Directories
BUILD_DIR := ./bin
INSTALL_DIR := $(HOME)/.local/bin

# Coverage
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html

.PHONY: all build install uninstall test test-coverage test-race lint fmt vet clean deps tidy help

## Default target
all: build

## Build the binary
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "✅ Built: $(BUILD_DIR)/$(BINARY_NAME)"

## Build for all platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "🔨 Building for Linux..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .

build-darwin:
	@echo "🔨 Building for macOS..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .

build-windows:
	@echo "🔨 Building for Windows..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .

## Install the binary to ~/.local/bin (user-writable, should be in PATH)
install: build
	@echo "📦 Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@chmod +x $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "✅ Installed: $(INSTALL_DIR)/$(BINARY_NAME)"
	@echo ""
	@echo "💡 Make sure $(INSTALL_DIR) is in your PATH:"
	@echo "   export PATH=\"\$$HOME/.local/bin:\$$PATH\""
	@echo ""
	@if ! echo "$$PATH" | grep -q "$(INSTALL_DIR)"; then \
		echo "⚠️  $(INSTALL_DIR) is NOT in your current PATH"; \
	else \
		echo "✅ $(INSTALL_DIR) is in your PATH"; \
	fi

## Install globally (requires sudo for /usr/local/bin)
install-global: build
	@echo "📦 Installing $(BINARY_NAME) to /usr/local/bin (requires sudo)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "✅ Installed: /usr/local/bin/$(BINARY_NAME)"

## Uninstall the binary
uninstall:
	@echo "🗑️  Uninstalling $(BINARY_NAME)..."
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@rm -f /usr/local/bin/$(BINARY_NAME) 2>/dev/null || true
	@echo "✅ Uninstalled"

## Run tests
test:
	@echo "🧪 Running tests..."
	$(GOTEST) -v ./...

## Run tests with coverage
test-coverage:
	@echo "🧪 Running tests with coverage..."
	$(GOTEST) -coverprofile=$(COVERAGE_FILE) ./...
	$(GOCMD) tool cover -func=$(COVERAGE_FILE)
	@echo ""
	@echo "📊 Total coverage: $$(go tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print $$3}')"

## Generate HTML coverage report
coverage-html: test-coverage
	@echo "📊 Generating HTML coverage report..."
	$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "✅ Coverage report: $(COVERAGE_HTML)"
	@command -v open >/dev/null && open $(COVERAGE_HTML) || echo "Open $(COVERAGE_HTML) in your browser"

## Run tests with race detection
test-race:
	@echo "🧪 Running tests with race detection..."
	$(GOTEST) -race -v ./...

## Run linters
lint:
	@echo "🔍 Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "⚠️  golangci-lint not found, running go vet only"; \
		$(GOVET) ./...; \
	fi

## Format code
fmt:
	@echo "🎨 Formatting code..."
	$(GOFMT) -s -w .
	@echo "✅ Code formatted"

## Run go vet
vet:
	@echo "🔍 Running go vet..."
	$(GOVET) ./...

## Download dependencies
deps:
	@echo "📥 Downloading dependencies..."
	$(GOMOD) download
	@echo "✅ Dependencies downloaded"

## Tidy dependencies
tidy:
	@echo "🧹 Tidying dependencies..."
	$(GOMOD) tidy
	@echo "✅ Dependencies tidied"

## Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@rm -f $(BINARY_NAME)
	@echo "✅ Cleaned"

## Run the analyzer on itself (dogfooding)
dogfood: build
	@echo "🐕 Running analyzer on itself..."
	$(BUILD_DIR)/$(BINARY_NAME) .

## Show help
help:
	@echo "temporal-analyzer - Temporal.io workflow analyzer for Go projects"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build the binary"
	@echo "  build-all      Build for all platforms (linux, darwin, windows)"
	@echo "  install        Install to ~/.local/bin (user-writable)"
	@echo "  install-global Install to /usr/local/bin (requires sudo)"
	@echo "  uninstall      Remove installed binary"
	@echo "  test           Run tests"
	@echo "  test-coverage  Run tests with coverage report"
	@echo "  coverage-html  Generate HTML coverage report"
	@echo "  test-race      Run tests with race detection"
	@echo "  lint           Run linters"
	@echo "  fmt            Format code"
	@echo "  vet            Run go vet"
	@echo "  deps           Download dependencies"
	@echo "  tidy           Tidy dependencies"
	@echo "  clean          Remove build artifacts"
	@echo "  dogfood        Run analyzer on itself"
	@echo "  help           Show this help"

