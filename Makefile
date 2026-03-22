.PHONY: help generate build test clean all install-tools deps

# Build CLI binaries - DEFAULT TARGET
build: generate
	@echo "Building CLI binaries..."
	mkdir -p ./bin
	go build -o ./bin/buildozer-client ./cmd/buildozer-client/main.go
	@echo "✓ Build complete: ./bin/buildozer-client"

# Download module dependencies
deps:
	@echo "Downloading module dependencies..."
	go mod download
	@echo "✓ Module dependencies downloaded"

# Install development tools (protoc plugins, buf, etc.)
install-tools: deps
	@echo "Installing development tools..."
	go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install github.com/bufbuild/buf/cmd/buf@v1.35.1
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "✓ Development tools installed"

# Generate code (protobuf via buf)
generate: install-tools
	@echo "Generating code..."
	go generate ./...
	@echo "✓ Code generation complete"

# Display help
help:
	@echo "Buildozer Makefile - Available targets:"
	@echo ""
	@echo "  build        - Build CLI binaries (buildozer-client) [DEFAULT]"
	@echo "  generate     - Run code generation (protobuf, etc.)"
	@echo "  install-tools- Install development tools (buf, protoc plugins)"
	@echo "  deps         - Download module dependencies"
	@echo "  test         - Run unit tests for all packages"
	@echo "  test-short   - Run unit tests in short mode (faster)"
	@echo "  test-verbose - Run unit tests with verbose output"
	@echo "  all          - Run generate, build, and test"
	@echo "  clean        - Clean build artifacts and generated code"
	@echo ""

# Run unit tests for all packages
test: generate
	@echo "Running unit tests..."
	go test -v ./...
	@echo "✓ All tests passed"

# Run unit tests in short mode (skip integration tests)
test-short: generate
	@echo "Running unit tests (short mode)..."
	go test -short -v ./...
	@echo "✓ Short tests passed"

# Run unit tests with verbose output
test-verbose: generate
	@echo "Running unit tests (verbose)..."
	go test -v -count=1 ./...
	@echo "✓ Verbose tests passed"

# Clean build artifacts and generated code
clean:
	@echo "Cleaning build artifacts..."
	rm -rf ./bin
	go clean ./...
	@echo "✓ Clean complete"

# Generate, build, and test everything
all: generate build test
	@echo "✓ All targets completed successfully"

# Verify project builds and passes basic checks
check: generate
	@echo "Checking project..."
	go build ./...
	go test -short ./...
	@echo "✓ Project checks passed"
