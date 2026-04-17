#!/bin/bash
# Build script for Buildozer CMake Example Project
# This script demonstrates building a CMake project using Buildozer drivers

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="${SCRIPT_DIR}/../.."
BUILD_DIR="${SCRIPT_DIR}/build"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Buildozer CMake Example Builder ===${NC}"
echo "Project directory: ${SCRIPT_DIR}"
echo "Build directory: ${BUILD_DIR}"
echo ""

# Check if cmake is available
if ! command -v cmake &> /dev/null; then
    echo -e "${RED}ERROR: cmake not found. Install cmake with: apt-get install cmake${NC}"
    exit 1
fi

# Create build directory
mkdir -p "${BUILD_DIR}"
cd "${BUILD_DIR}"

echo -e "${YELLOW}Running CMake configuration...${NC}"
cmake -DCMAKE_VERBOSE_MAKEFILE=ON ..

echo -e "${YELLOW}Building all targets...${NC}"
cmake --build . --verbose

echo ""
echo -e "${GREEN}✓ Build completed successfully!${NC}"
echo ""
echo "Executables created:"
ls -lh bin/

echo ""
echo -e "${YELLOW}To run the executables:${NC}"
echo "  C calculator:   ./bin/calc_c"
echo "  C++ calculator: ./bin/calc_cpp"
echo ""
echo "Or use CMake targets:"
echo "  cmake --build . --target run_calc_c"
echo "  cmake --build . --target run_calc_cpp"
