.PHONY: help generate build test clean all

# Generate code (protobuf via buf) - DEFAULT TARGET
generate:
	@echo "Generating code..."
	go generate ./...
	@echo "✓ Code generation complete"

# Display help
help:
	@echo "Buildozer Makefile - Available targets:"
	@echo ""
	@echo "  generate     - Run code generation (protobuf, etc.) [DEFAULT]"
	@echo "  build        - Build CLI binaries (buildozer-client)"
	@echo "  test         - Run unit tests for all packages"
	@echo "  test-short   - Run unit tests in short mode (faster)"
	@echo "  test-verbose - Run unit tests with verbose output"
	@echo "  all          - Run generate, build, and test"
	@echo "  clean        - Clean build artifacts and generated code"
	@echo ""

# Build CLI binaries
build: generate
	@echo "Building CLI binaries..."
	mkdir -p ./bin
	go build -o ./bin/buildozer-client ./cmd/buildozer-client/main.go
	@echo "✓ Build complete: ./bin/buildozer-client"

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

# Build without tests (faster for development)
build-only: generate
	@echo "Building CLI binaries (no tests)..."
	mkdir -p ./bin
	go build -o ./bin/buildozer-client ./cmd/buildozer-client/main.go
	@echo "✓ Build complete: ./bin/buildozer-client"

# Verify project builds and passes basic checks
check: generate
	@echo "Checking project..."
	go build ./...
	go test -short ./...
	@echo "✓ Project checks passed"
