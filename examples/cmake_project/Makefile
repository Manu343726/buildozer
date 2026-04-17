# Makefile for Buildozer CMake Example Project
# 
# This Makefile:
# 1. Compiles Buildozer drivers (gcc, g++) if needed
# 2. Configures the CMake project with these drivers
# 3. Builds the CMake project
#
# Usage:
#   make              - Build everything (drivers + cmake project)
#   make clean        - Remove build artifacts
#   make rebuild      - Clean and rebuild everything
#   make run          - Run the example executables
#   make run_c        - Run the C calculator
#   make run_cpp      - Run the C++ calculator

.PHONY: all build config clean rebuild run run_c run_cpp help drivers

# Project directories
PROJECT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
PARENT_DIR := $(abspath $(PROJECT_DIR)../..)
BUILD_DIR := $(PROJECT_DIR)build
DRIVERS_DIR := $(PARENT_DIR)/bin

# Buildozer driver paths
GCC_DRIVER := $(DRIVERS_DIR)/gcc
GXX_DRIVER := $(DRIVERS_DIR)/g++

# CMake settings
CMAKE_FLAGS := -DCMAKE_VERBOSE_MAKEFILE=ON

# Compiler for building the drivers
GO := go
MAKE_DRIVER := make

# Color output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
NC := \033[0m

# Default target
all: print_header drivers config build

print_header:
	@echo "$(BLUE)=== Buildozer CMake Example Builder ===$(NC)"
	@echo "Project directory: $(PROJECT_DIR)"
	@echo "Build directory: $(BUILD_DIR)"
	@echo "Drivers directory: $(DRIVERS_DIR)"
	@echo ""

# Ensure drivers are built
drivers: $(GCC_DRIVER) $(GXX_DRIVER)

$(GCC_DRIVER) $(GXX_DRIVER):
	@echo "$(YELLOW)Building Buildozer drivers...$(NC)"
	@cd $(PARENT_DIR) && $(MAKE_DRIVER) build
	@if [ ! -f $(GCC_DRIVER) ] || [ ! -f $(GXX_DRIVER) ]; then \
		echo "$(RED)ERROR: Failed to build drivers$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✓ Drivers built successfully$(NC)"

# Configure CMake project (depends on drivers being built)
config: drivers check_cmake print_drivers
	@echo "$(YELLOW)Creating build directory...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@echo "$(YELLOW)Running CMake configuration...$(NC)"
	@cd $(BUILD_DIR) && cmake $(CMAKE_FLAGS) ..
	@echo "$(GREEN)✓ CMake configuration complete$(NC)"
	@echo ""

check_cmake:
	@if ! command -v cmake >/dev/null 2>&1; then \
		echo "$(RED)ERROR: cmake not found. Install with: apt-get install cmake$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✓ cmake found: $$(cmake --version | head -n1)$(NC)"

print_drivers:
	@echo "$(GREEN)✓ GCC driver: $(GCC_DRIVER)$(NC)"
	@echo "$(GREEN)✓ G++ driver: $(GXX_DRIVER)$(NC)"
	@echo ""

# Build the CMake project
build: config
	@echo "$(YELLOW)Building CMake project...$(NC)"
	@cd $(BUILD_DIR) && cmake --build . --verbose
	@echo ""
	@echo "$(GREEN)✓ Build completed successfully!$(NC)"
	@echo ""
	@if [ -f $(BUILD_DIR)/bin/calc_c ] && [ -f $(BUILD_DIR)/bin/calc_cpp ]; then \
		echo "$(GREEN)Executables created:$(NC)"; \
		ls -lh $(BUILD_DIR)/bin/; \
		echo ""; \
	fi

# Clean build artifacts (keep drivers)
clean:
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -rf $(BUILD_DIR)
	@echo "$(GREEN)✓ Clean complete$(NC)"
	@echo ""

# Full rebuild (clean + build)
rebuild: clean build

# Run both executables
run: build run_c run_cpp

# Run C calculator
run_c: build
	@echo "$(BLUE)=== Running C Calculator ===$(NC)"
	@echo ""
	@$(BUILD_DIR)/bin/calc_c
	@echo ""

# Run C++ calculator
run_cpp: build
	@echo "$(BLUE)=== Running C++ Calculator ===$(NC)"
	@echo ""
	@$(BUILD_DIR)/bin/calc_cpp
	@echo ""

# Show help
help:
	@echo "$(BLUE)Buildozer CMake Example Project - Makefile Targets$(NC)"
	@echo ""
	@echo "$(YELLOW)Main targets:$(NC)"
	@echo "  all              - Build drivers and CMake project (default)"
	@echo "  build            - Build CMake project (drivers compiled if needed)"
	@echo "  config           - Configure CMake (compiles drivers if needed)"
	@echo "  drivers          - Build Buildozer drivers only"
	@echo "  clean            - Remove build artifacts (keep drivers)"
	@echo "  rebuild          - Clean and rebuild everything"
	@echo ""
	@echo "$(YELLOW)Run targets:$(NC)"
	@echo "  run              - Run all executables"
	@echo "  run_c            - Run C calculator"
	@echo "  run_cpp          - Run C++ calculator"
	@echo ""
	@echo "$(YELLOW)Info:$(NC)"
	@echo "  help             - Show this help message"
	@echo ""
	@echo "$(YELLOW)Examples:$(NC)"
	@echo "  make              # Build everything"
	@echo "  make rebuild      # Full rebuild"
	@echo "  make run_c        # Run just C calculator"
	@echo "  make clean build  # Clean and rebuild"
	@echo ""
