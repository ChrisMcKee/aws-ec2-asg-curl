SHELL := /bin/bash

GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOMOD=$(GOCMD) mod
BINARY_NAME=ec2asgcurl
MAIN_PACKAGE=./

LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"
VERSION ?= $(shell git describe --tags --always --dirty)
BUILD_FLAGS=-trimpath $(LDFLAGS)

BUILD_DIR=build
DIST_DIR=dist

OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

GOPATH := $(shell go env GOPATH)
GOLANGCI_LINT_PATH := $(GOPATH)/bin/golangci-lint
GOFUMPT_PATH := $(GOPATH)/bin/gofumpt

.PHONY: all build clean test coverage lint fmt mod-tidy install-tools cross-build tag-release

all: clean lint test build # Default target

build: # Build the application
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

clean: # Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	go clean -testcache

test: # Run tests
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

coverage: # Generate test coverage report
	@echo "Generating coverage report..."
	@mkdir -p $(BUILD_DIR)
	$(GOTEST) -coverprofile=$(BUILD_DIR)/coverage.out ./...
	$(GOCMD) tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "Coverage report generated at $(BUILD_DIR)/coverage.html"

lint: # Run linter
	@if [ ! -f "$(GOLANGCI_LINT_PATH)" ]; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@echo "Running linter..."
	@$(GOLANGCI_LINT_PATH) run ./...

fmt: # Format code
	@if [ ! -f "$(GOFUMPT_PATH)" ]; then \
		echo "Installing gofumpt..."; \
		go install mvdan.cc/gofumpt@latest; \
	fi
	@echo "Formatting code..."
	@$(GOFUMPT_PATH) -l -w .

mod-tidy: # Update Go modules
	@echo "Updating Go modules..."
	$(GOMOD) tidy
	$(GOMOD) verify

install-tools: # Install development tools
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install mvdan.cc/gofumpt@latest
	@go install golang.org/x/tools/cmd/goimports@latest

cross-build: # Cross-build for all platforms
	@echo "Cross-building for multiple platforms..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		OS=$${platform%/*} ; \
		ARCH=$${platform#*/} ; \
		echo "Building for $$OS/$$ARCH..." ; \
		if [ "$$OS" = "windows" ]; then \
			EXTENSION=".exe" ; \
		else \
			EXTENSION="" ; \
		fi ; \
		GOOS=$$OS GOARCH=$$ARCH $(GOBUILD) $(BUILD_FLAGS) \
			-o $(DIST_DIR)/$(BINARY_NAME)_$${OS}_$${ARCH}$${EXTENSION} $(MAIN_PACKAGE) ; \
	done

run: # Run the application (for development)
	$(GOCMD) run $(MAIN_PACKAGE) $(ARGS)

install: # Install the application locally
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(BUILD_FLAGS) $(MAIN_PACKAGE)

tag-release: # Prompt for version and create a signed, annotated tag
	@read -p "Enter the version name (e.g., v1.2.3): " version; \
	if [ -z "$$version" ]; then \
		echo "Error: Version name cannot be empty."; \
		exit 1; \
	fi; \
	echo "Creating signed tag: $$version"; \
	git tag -s -a "$$version" -m "$$version"; \
	if [ $$? -eq 0 ]; then \
		echo "Successfully tagged version $$version."; \
		echo "Remember to push the tag: git push origin $$version"; \
	else \
		echo "Error: Failed to create tag $$version."; \
		exit 1; \
	fi

help: # Show help
	@echo "Available targets:"
	@echo "  all          - Clean, lint, test, and build"
	@echo "  build        - Build the application"
	@echo "  clean        - Remove build artifacts"
	@echo "  test         - Run tests"
	@echo "  coverage     - Generate test coverage report"
	@echo "  lint         - Run linter"
	@echo "  fmt          - Format code"
	@echo "  mod-tidy     - Update Go modules"
	@echo "  install-tools - Install development tools"
	@echo "  cross-build  - Build for multiple platforms"
	@echo "  run          - Run the application (use ARGS=\"<args>\" for arguments)"
	@echo "  install      - Install the application locally"
	@echo "  tag-release  - Create a signed, annotated Git tag"
	@echo "  help         - Show this help message"

.DEFAULT_GOAL := help 
