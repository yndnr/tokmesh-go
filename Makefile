# TokMesh 项目 Makefile

# Go 参数
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# 项目参数
BINARY_NAME=tokmesh
BINARY_DIR=bin
PROTO_DIR=internal/storage/proto
PROTO_OUT_DIR=internal/storage/proto

# 版本信息
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

.PHONY: all build test clean proto fmt lint help install deps tidy

# 默认目标
all: fmt lint test build

# 编译
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/tokmesh

# 运行测试
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Test coverage:"
	@$(GOCMD) tool cover -func=coverage.out | grep total | awk '{print "Total: " $$3}'

# 运行测试（快速模式，不含 race detector）
test-fast:
	@echo "Running tests (fast mode)..."
	$(GOTEST) -v ./...

# 测试覆盖率报告
coverage:
	@echo "Generating coverage report..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# 基准测试
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# 生成 Protobuf 代码
proto:
	@echo "Generating protobuf code..."
	@./scripts/proto-gen.sh

# 格式化代码
fmt:
	@echo "Formatting code..."
	@$(GOFMT) -w -s .

# 代码检查
lint:
	@echo "Linting code..."
	@if command -v $(GOLINT) >/dev/null 2>&1; then \
		$(GOLINT) run ./...; \
	else \
		echo "golangci-lint not installed, skipping lint..."; \
		echo "Install with: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin"; \
	fi

# 安装依赖
deps:
	@echo "Installing dependencies..."
	$(GOGET) -u google.golang.org/protobuf/cmd/protoc-gen-go
	$(GOMOD) download

# 整理依赖
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# 清理
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BINARY_DIR)
	@rm -f coverage.out coverage.html
	@rm -f internal/storage/proto/*.pb.go

# 安装
install: build
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BINARY_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

# 运行
run: build
	@echo "Running $(BINARY_NAME)..."
	@$(BINARY_DIR)/$(BINARY_NAME)

# 帮助信息
help:
	@echo "TokMesh Makefile Commands:"
	@echo ""
	@echo "  make build       - 编译项目"
	@echo "  make test        - 运行所有测试（含 race detector）"
	@echo "  make test-fast   - 运行测试（快速模式）"
	@echo "  make coverage    - 生成测试覆盖率报告"
	@echo "  make bench       - 运行基准测试"
	@echo "  make proto       - 生成 Protobuf 代码"
	@echo "  make fmt         - 格式化代码"
	@echo "  make lint        - 代码检查"
	@echo "  make deps        - 安装依赖"
	@echo "  make tidy        - 整理依赖"
	@echo "  make clean       - 清理构建产物"
	@echo "  make install     - 安装到 GOPATH/bin"
	@echo "  make run         - 运行程序"
	@echo "  make all         - 执行 fmt + lint + test + build"
	@echo "  make help        - 显示此帮助信息"
