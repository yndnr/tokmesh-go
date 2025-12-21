.PHONY: build build-linux build-windows build-darwin docker docker-build docker-push test clean install help

# Version from git tags or commit hash
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags
LDFLAGS := -X github.com/yndnr/tokmesh-go/internal/infra/buildinfo.Version=$(VERSION) \
           -X github.com/yndnr/tokmesh-go/internal/infra/buildinfo.CommitHash=$(COMMIT) \
           -X github.com/yndnr/tokmesh-go/internal/infra/buildinfo.BuildTime=$(BUILD_TIME)

# Output directory
BIN_DIR := bin

# Docker settings
DOCKER_REGISTRY ?= tokmesh
DOCKER_IMAGE := $(DOCKER_REGISTRY)/tokmesh-server
DOCKER_TAG ?= $(VERSION)

## Build targets

build: ## Build binaries for current platform
	@echo "Building for current platform..."
	@mkdir -p $(BIN_DIR)
	cd src && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o ../$(BIN_DIR)/tokmesh-server ./cmd/tokmesh-server
	cd src && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o ../$(BIN_DIR)/tokmesh-cli ./cmd/tokmesh-cli
	@echo "✓ Build complete: $(BIN_DIR)/"

build-linux: ## Build Linux binaries (amd64 and arm64)
	@echo "Building for Linux..."
	@mkdir -p $(BIN_DIR)
	cd src && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" \
		-o ../$(BIN_DIR)/tokmesh-server-linux-amd64 ./cmd/tokmesh-server
	cd src && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" \
		-o ../$(BIN_DIR)/tokmesh-cli-linux-amd64 ./cmd/tokmesh-cli
	cd src && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" \
		-o ../$(BIN_DIR)/tokmesh-server-linux-arm64 ./cmd/tokmesh-server
	cd src && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" \
		-o ../$(BIN_DIR)/tokmesh-cli-linux-arm64 ./cmd/tokmesh-cli
	@echo "✓ Linux builds complete"

build-windows: ## Build Windows binaries
	@echo "Building for Windows..."
	@mkdir -p $(BIN_DIR)
	cd src && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" \
		-o ../$(BIN_DIR)/tokmesh-server.exe ./cmd/tokmesh-server
	cd src && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" \
		-o ../$(BIN_DIR)/tokmesh-cli.exe ./cmd/tokmesh-cli
	@echo "✓ Windows builds complete"

build-darwin: ## Build macOS binaries
	@echo "Building for macOS..."
	@mkdir -p $(BIN_DIR)
	cd src && GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" \
		-o ../$(BIN_DIR)/tokmesh-server-darwin-amd64 ./cmd/tokmesh-server
	cd src && GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" \
		-o ../$(BIN_DIR)/tokmesh-cli-darwin-amd64 ./cmd/tokmesh-cli
	cd src && GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" \
		-o ../$(BIN_DIR)/tokmesh-server-darwin-arm64 ./cmd/tokmesh-server
	cd src && GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" \
		-o ../$(BIN_DIR)/tokmesh-cli-darwin-arm64 ./cmd/tokmesh-cli
	@echo "✓ macOS builds complete"

build-all: build-linux build-windows build-darwin ## Build for all platforms

## Docker targets

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		.
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	@echo "✓ Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

docker-push: docker-build ## Build and push Docker image
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest
	@echo "✓ Docker image pushed"

docker: docker-build ## Alias for docker-build

## Test targets

test: ## Run tests
	@echo "Running tests..."
	cd src && go test -race -cover ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	cd src && go test -race -coverprofile=coverage.out ./...
	cd src && go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: src/coverage.html"

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	cd src && go test -race -tags=integration ./internal/tests/...

## Code quality targets

lint: ## Run linters
	@echo "Running linters..."
	cd src && go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && cd src && golangci-lint run || echo "golangci-lint not installed, skipping"

fmt: ## Format code
	@echo "Formatting code..."
	cd src && go fmt ./...
	cd src && gofmt -s -w .

tidy: ## Tidy go.mod
	@echo "Tidying dependencies..."
	cd src && go mod tidy

## Install targets

install: build ## Install binaries to /usr/local/bin
	@echo "Installing binaries..."
	sudo install -m 755 $(BIN_DIR)/tokmesh-server /usr/local/bin/
	sudo install -m 755 $(BIN_DIR)/tokmesh-cli /usr/local/bin/
	@echo "✓ Installed to /usr/local/bin/"

uninstall: ## Uninstall binaries from /usr/local/bin
	@echo "Uninstalling binaries..."
	sudo rm -f /usr/local/bin/tokmesh-server
	sudo rm -f /usr/local/bin/tokmesh-cli
	@echo "✓ Uninstalled"

## Utility targets

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)/
	cd src && go clean
	@echo "✓ Clean complete"

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Built:   $(BUILD_TIME)"

help: ## Show this help message
	@echo "TokMesh Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'
