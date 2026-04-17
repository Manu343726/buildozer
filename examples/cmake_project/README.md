# Buildozer CMake Example Project

This example demonstrates a real-world CMake project that uses the Buildozer `gcc` and `g++` drivers as C and C++ compilers.

## Project Overview

The example consists of a simple calculator application that demonstrates:

1. **C Compilation** (`gcc` driver)
   - Pure C math library (`src/math.c`)
   - C executable using the math library (`src/main_c.c`)

2. **C++ Compilation** (`g++` driver)
   - C++ calculator class that wraps C math functions (`src/Calculator.cpp`)
   - C++ executable using the calculator class (`src/main_cpp.cpp`)

3. **C/C++ Interoperability**
   - C++ code calling C library functions through `extern "C"`
   - CMake configuration to build mixed C/C++ projects
   - Linking between C and C++ object files

## Directory Structure

```
examples/cmake_project/
├── .buildozer              # Buildozer configuration for this project
├── CMakeLists.txt          # CMake build configuration
├── build.sh                # Helper build script
├── README.md               # This file
├── include/
│   ├── math.h              # C math library header
│   └── Calculator.hpp      # C++ calculator class header
├── src/
│   ├── math.c              # C math library implementation
│   ├── Calculator.cpp      # C++ calculator implementation
│   ├── main_c.c            # C example program
│   └── main_cpp.cpp        # C++ example program
└── build/                  # Build output directory (generated)
    ├── bin/                # Compiled executables
    └── lib/                # Compiled libraries
```

## Requirements

### Prerequisites

- CMake 3.10 or higher
- Buildozer daemon running on `127.0.0.1:6789` (the default port)
- The Buildozer project built in the parent directory (`../../bin/gcc` and `../../bin/g++` should exist)

```bash
# Install CMake on Ubuntu/Debian
sudo apt-get install cmake

# Start the Buildozer daemon (in another terminal)
cd /path/to/buildozer
./bin/buildozer-client daemon
```

### Buildozer Daemon

This example project communicates with the Buildozer daemon to submit compilation jobs. You can run the daemon locally by:

```bash
# From the buildozer project root
./bin/buildozer-client daemon
```

The daemon will listen on `127.0.0.1:6789` by default (configurable via `.buildozer` config).

## Building the Project

### Step 1: Start the Buildozer Daemon

Before building, ensure the Buildozer daemon is running:

```bash
# From the buildozer project root directory
./bin/buildozer-client daemon
```

You can verify the daemon is running:
```bash
./bin/buildozer-client status
```

### Step 2: Build Using Make

The project includes a Makefile that automatically builds the Buildozer drivers (if needed) and configures the CMake project:

```bash
cd examples/cmake_project
make              # Build everything (drivers + cmake project)
```

The Makefile ensures that:
- Buildozer drivers (`gcc` and `g++`) are compiled if needed
- CMake is configured with the correct compiler paths
- The project is built with verbose output

### Other Make Targets

```bash
make rebuild      # Clean and rebuild everything
make clean        # Remove build artifacts (keep drivers)
make run          # Run all executables
make run_c        # Run C calculator only
make run_cpp      # Run C++ calculator only
make help         # Show all available targets
```

### Build Output

After successful build, you'll have:
- `build/bin/calc_c` - Pure C calculator executable
- `build/bin/calc_cpp` - C++ calculator executable
- `build/lib/libmath_c.a` - C static library
- `build/lib/libmath_cpp.a` - C++ static library

## Running the Examples

### C Calculator

The C calculator uses the pure C math library:

```bash
./build/bin/calc_c
```

Expected output:
```
=== C Calculator (compiled with Buildozer GCC) ===
Using Buildozer gcc driver for C compilation

Test 1: Basic Arithmetic
  42 + 8 = 50
  42 - 8 = 34
  42 * 8 = 336
  42 / 8 = 5

Test 2: Power Function
  2^0 = 1
  2^1 = 2
  2^2 = 4
  2^3 = 8
  2^4 = 16

...
✓ C calculator completed successfully
```

### C++ Calculator

The C++ calculator uses the Calculator class that wraps the C math library:

```bash
./build/bin/calc_cpp
```

Expected output:
```
=== C++ Calculator (compiled with Buildozer G++) ===
Using Buildozer g++ driver for C++ compilation
Calculator calls C math library (gcc-compiled)

[Calculator] Initialized (compiled with Buildozer g++)
Test 1: Basic Operations
  100 + 50 = 150
  150 - 25 = 125
  125 * 2 = 250
  250 / 4 = 62
...
✓ C++ calculator completed successfully
```

### Using CMake Targets

You can also run using CMake custom targets from the build directory:

```bash
cd build
cmake --build . --target run_calc_c
cmake --build . --target run_calc_cpp
```

Or use the convenient make targets:

```bash
make run_c
make run_cpp
```

## Configuration

The `.buildozer` file in this directory contains Buildozer-specific configuration:

```yaml
drivers:
  gcc:
    compiler_version: "10.2.1"  # Full semantic version
    c_runtime: "glibc"
    c_runtime_version: "2.31"
    architecture: "x86_64"

  g++:
    compiler_version: "10.2.1"
    c_runtime: "glibc"
    c_runtime_version: "2.31"
    cpp_stdlib: "libstdc++"
    architecture: "x86_64"
```

**Note:** This project uses `standalone: true` mode, meaning each driver compiles locally without distributing to peers.

## CMakeLists.txt Details

The CMakeLists.txt demonstrates:

1. **Setting Buildozer drivers as compilers:**
   ```cmake
   set(CMAKE_C_COMPILER "${CMAKE_CURRENT_SOURCE_DIR}/../../../bin/gcc")
   set(CMAKE_CXX_COMPILER "${CMAKE_CURRENT_SOURCE_DIR}/../../../bin/g++")
   ```

2. **Building C and C++ libraries separately:**
   ```cmake
   add_library(math_c src/math.c)
   add_library(math_cpp src/Calculator.cpp)
   target_link_libraries(math_cpp PUBLIC math_c)
   ```

3. **Creating mixed-language executables:**
   ```cmake
   add_executable(calc_c src/main_c.c)
   target_link_libraries(calc_c PRIVATE math_c)

   add_executable(calc_cpp src/main_cpp.cpp)
   target_link_libraries(calc_cpp PRIVATE math_cpp)
   ```

## Testing the Drivers

This example can be used to verify:

1. ✓ **Buildozer GCC driver** directly compiles C source code
2. ✓ **Buildozer G++ driver** directly compiles C++ source code
3. ✓ **Linking** between C and C++ object files works correctly
4. ✓ **C/C++ Interoperability** with `extern "C"` function declarations
5. ✓ **Real CMake projects** work with Buildozer drivers as system compilers
6. ✓ **Compiler detection** - CMake can find and use the drivers

## Troubleshooting

### CMake Cannot Find Compiler

**Error:** `CMake Error at CMakeLists.txt:X (project): No CMAKE_C_COMPILER could be found.`

**Solution:** Ensure the Buildozer binaries exist at the paths specified in CMakeLists.txt:
```bash
ls -la ../../bin/gcc
ls -la ../../bin/g++
```

### Build Fails with Compilation Error

**Error:** `error: command not found` or similar

**Solution:** The Buildozer drivers need the actual compilers to be available. Ensure GCC and G++ are installed:
```bash
sudo apt-get install build-essential
```

### Permission Denied

**Error:** `Permission denied: '../../../bin/gcc'`

**Solution:** Ensure the binaries have execute permissions:
```bash
chmod +x ../../bin/gcc ../../bin/g++
```

## Real-World Scenarios

This example demonstrates how Buildozer drivers can be used in:

1. **Existing CMake projects** - Replace system compilers without modifying build files (just adjust paths)
2. **Mixed language projects** - Coordinating C and C++ compilation
3. **Library building** - Creating static/shared libraries with consistent compiler settings
4. **CI/CD pipelines** - Using Buildozer drivers in automated builds
5. **Cross-compilation** - The drivers abstract away compiler complexity

## Next Steps

After verifying this example works:

1. Try modifying the source code and rebuilding
2. Add more complex C/C++ code to test different scenarios
3. Experiment with different compiler flags in CMakeLists.txt
4. Test with distributed mode enabled in `.buildozer` to submit jobs to Buildozer daemon
5. Use this as a template for testing your own CMake projects

## Contributing

To add more examples:
1. Create a new subdirectory under `examples/`
2. Include CMakeLists.txt, source files, and a README
3. Test with both standalone and distributed modes
4. Document any special considerations

## See Also

- [Buildozer Configuration Guide](../../.buildozer)
- [CMake Documentation](https://cmake.org/documentation/)
- [GCC Documentation](https://gcc.gnu.org/onlinedocs/)
- [G++ Documentation](https://gcc.gnu.org/onlinedocs/)
