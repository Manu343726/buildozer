cmake_minimum_required(VERSION 3.10)
project(CalculatorApp C CXX)

message(STATUS "C Compiler: ${CMAKE_C_COMPILER}")
message(STATUS "CXX Compiler: ${CMAKE_CXX_COMPILER}")

# Set C and C++ standards
set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Output directories
set(CMAKE_RUNTIME_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/bin")
set(CMAKE_LIBRARY_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/lib")
set(CMAKE_ARCHIVE_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/lib")

# Include directories
include_directories(${CMAKE_CURRENT_SOURCE_DIR}/include)

# C Library - math_c (using C compiler)
add_library(math_c src/math.c)
set_target_properties(math_c PROPERTIES
  LANGUAGE C
)

# C++ Library - math_cpp (using C++ compiler with linked C library)
add_library(math_cpp src/Calculator.cpp)
target_link_libraries(math_cpp PUBLIC math_c)
set_target_properties(math_cpp PROPERTIES
  LANGUAGE CXX
)

# Executables
# Pure C program
add_executable(calc_c src/main_c.c)
target_link_libraries(calc_c PRIVATE math_c)
set_target_properties(calc_c PROPERTIES
  LANGUAGE C
)

# C++ program
add_executable(calc_cpp src/main_cpp.cpp)
target_link_libraries(calc_cpp PRIVATE math_cpp)
set_target_properties(calc_cpp PROPERTIES
  LANGUAGE CXX
)

# Add custom targets for testing
add_custom_target(run_calc_c
  COMMAND ${CMAKE_RUNTIME_OUTPUT_DIRECTORY}/calc_c
  COMMENT "Running C calculator"
  DEPENDS calc_c
)

add_custom_target(run_calc_cpp
  COMMAND ${CMAKE_RUNTIME_OUTPUT_DIRECTORY}/calc_cpp
  COMMENT "Running C++ calculator"
  DEPENDS calc_cpp
)

# Print summary
message(STATUS "")
message(STATUS "=== Buildozer Calculator Example ===")
message(STATUS "Components configured:")
message(STATUS "  - C Library: math_c")
message(STATUS "  - C++ Library: math_cpp")
message(STATUS "  - C Executable: calc_c")
message(STATUS "  - C++ Executable: calc_cpp")
message(STATUS "")
message(STATUS "Build output directory: ${CMAKE_RUNTIME_OUTPUT_DIRECTORY}")
message(STATUS "")
