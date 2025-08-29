# JSFinder Makefile
# Go cybersecurity tool for crawling domains and finding secrets in JavaScript files

# Variables
APP_NAME := jsfinder
VERSION := 1.0.0
BUILD_DIR := build
BIN_DIR := bin
GO_FILES := $(shell find . -name '*.go' -type f -not -path './vendor/*')
GO_PACKAGES := $(shell go list ./...)

# Build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)"
BUILD_FLAGS := -v $(LDFLAGS)

# Default target
.PHONY: all
all: clean build

# Build the application
.PHONY: build
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	go build $(BUILD_FLAGS) -o $(BIN_DIR)/$(APP_NAME) .
	@echo "Build complete: $(BIN_DIR)/$(APP_NAME)"

# Build for multiple platforms
.PHONY: build-all
build-all: clean
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 .
	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 .
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 .
	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 .
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe .
	@echo "Multi-platform build complete"

# Install the application
.PHONY: install
install: build
	@echo "Installing $(APP_NAME)..."
	go install $(BUILD_FLAGS) .
	@echo "Installation complete"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "Tests complete"

# Run tests with coverage report
.PHONY: test-coverage
test-coverage: test
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
.PHONY: bench
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Lint the code
.PHONY: lint
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run; \
	fi

# Format the code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w $(GO_FILES)

# Vet the code
.PHONY: vet
vet:
	@echo "Vetting code..."
	go vet ./...

# Run security checks
.PHONY: security
security:
	@echo "Running security checks..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not found, installing..."; \
		go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; \
		gosec ./...; \
	fi

# Check for vulnerabilities
.PHONY: vuln-check
vuln-check:
	@echo "Checking for vulnerabilities..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "govulncheck not found, installing..."; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
		govulncheck ./...; \
	fi

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BIN_DIR) $(BUILD_DIR)
	rm -f coverage.out coverage.html
	go clean -cache
	@echo "Clean complete"

# Download dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies updated"

# Update dependencies
.PHONY: deps-update
deps-update:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy
	@echo "Dependencies updated"

# Run the application with sample commands
.PHONY: run
run: build
	@echo "Running $(APP_NAME) help..."
	./$(BIN_DIR)/$(APP_NAME) --help

# Run crawl command example
.PHONY: run-crawl
run-crawl: build
	@echo "Running crawl example..."
	./$(BIN_DIR)/$(APP_NAME) crawl --domain https://example.com --output example-js.txt --verbose

# Run scan command example
.PHONY: run-scan
run-scan: build
	@echo "Running scan example..."
	@echo "https://cdnjs.cloudflare.com/ajax/libs/jquery/3.6.0/jquery.min.js" | ./$(BIN_DIR)/$(APP_NAME) scan --format json --verbose

# Run discover command example
.PHONY: run-discover
run-discover: build
	@echo "Running discover example..."
	@echo "https://cdnjs.cloudflare.com/ajax/libs/jquery/3.6.0/jquery.min.js" | ./$(BIN_DIR)/$(APP_NAME) discover --wordlist config/endpoints.txt --verbose

# Development setup
.PHONY: dev-setup
dev-setup:
	@echo "Setting up development environment..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Development environment setup complete"

# Create release
.PHONY: release
release: clean test lint security build-all
	@echo "Creating release $(VERSION)..."
	@mkdir -p $(BUILD_DIR)/release
	@for binary in $(BUILD_DIR)/$(APP_NAME)-*; do \
		if [ -f "$$binary" ]; then \
			basename=$$(basename $$binary); \
			tar -czf $(BUILD_DIR)/release/$$basename.tar.gz -C $(BUILD_DIR) $$basename; \
		fi; \
	done
	@echo "Release $(VERSION) created in $(BUILD_DIR)/release/"

# Docker build
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):$(VERSION) .
	docker tag $(APP_NAME):$(VERSION) $(APP_NAME):latest
	@echo "Docker image built: $(APP_NAME):$(VERSION)"

# Docker run
.PHONY: docker-run
docker-run:
	@echo "Running Docker container..."
	docker run --rm -it $(APP_NAME):latest --help

# Show help
.PHONY: help
help:
	@echo "JSFinder Makefile Commands:"
	@echo ""
	@echo "Build Commands:"
	@echo "  build        - Build the application"
	@echo "  build-all    - Build for multiple platforms"
	@echo "  install      - Install the application"
	@echo "  clean        - Clean build artifacts"
	@echo ""
	@echo "Development Commands:"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage"
	@echo "  bench        - Run benchmarks"
	@echo "  lint         - Run linters"
	@echo "  fmt          - Format code"
	@echo "  vet          - Vet code"
	@echo "  security     - Run security checks"
	@echo "  vuln-check   - Check for vulnerabilities"
	@echo ""
	@echo "Dependency Commands:"
	@echo "  deps         - Download dependencies"
	@echo "  deps-update  - Update dependencies"
	@echo ""
	@echo "Run Commands:"
	@echo "  run          - Run application help"
	@echo "  run-crawl    - Run crawl example"
	@echo "  run-scan     - Run scan example"
	@echo "  run-discover - Run discover example"
	@echo ""
	@echo "Setup Commands:"
	@echo "  dev-setup    - Setup development environment"
	@echo "  release      - Create release"
	@echo ""
	@echo "Docker Commands:"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo ""
	@echo "Other Commands:"
	@echo "  help         - Show this help message"