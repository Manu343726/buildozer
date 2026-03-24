.PHONY: help generate build build-tests test test-short test-verbose clean all install-tools deps check

# Build CLI binaries - DEFAULT TARGET
build: generate build-tests
	@echo "Building CLI binaries..."
	mkdir -p ./bin
	go build -o ./bin/buildozer-client ./cmd/buildozer-client/main.go
	go build -o ./bin/gcc ./cmd/drivers/cpp/gcc
	go build -o ./bin/g++ ./cmd/drivers/cpp/gxx
	go build -o ./bin/clang ./cmd/drivers/cpp/clang
	go build -o ./bin/clang++ ./cmd/drivers/cpp/clangxx
	@echo "✓ Build complete: ./bin/buildozer-client ./bin/gcc ./bin/g++ ./bin/clang ./bin/clang++"

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
	@echo "  build-tests  - Compile all test files without running them"
	@echo "  test         - Run unit tests for all packages"
	@echo "  test-short   - Run unit tests in short mode (faster)"
	@echo "  test-verbose - Run unit tests with verbose output"
	@echo "  all          - Run generate, build, and test"
	@echo "  clean        - Clean build artifacts and generated code"
	@echo ""

# Build all test files (compile tests without running them)
build-tests: generate
	@echo "Building test files..."
	@for dir in $$(find . -name '*_test.go' -type f | xargs dirname | sort -u | grep -v vendor); do \
		echo "  Testing $$dir..."; \
		go test -c -o /dev/null $$dir || exit 1; \
	done
	@echo "✓ All test files compiled successfully"

# Run unit tests for all packages
test: generate build-tests
	@echo "Running unit tests..."
	go test -v ./...
	@echo "✓ All tests passed"

# Run unit tests in short mode (skip integration tests)
test-short: generate build-tests
	@echo "Running unit tests (short mode)..."
	go test -short -v ./...
	@echo "✓ Short tests passed"

# Run unit tests with verbose output
test-verbose: generate build-tests
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
