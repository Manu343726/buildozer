// Package main provides a utility to display available C/C++ toolchains on the system.
// This example demonstrates how drivers and runtimes can use the toolchain detection API.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/Manu343726/buildozer/pkg/toolchain"
)

func main() {
	flag.Parse()

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get the global toolchain registry
	registry := toolchain.Global()

	// Initialize detection
	fmt.Println("Detecting available C/C++ toolchains...")
	if err := registry.Initialize(ctx); err != nil {
		log.Fatalf("Failed to detect toolchains: %v", err)
	}

	// Get all toolchains
	toolchains := registry.ListToolchains()
	if len(toolchains) == 0 {
		fmt.Println("No C/C++ toolchains detected on this system")
		return
	}

	// Display results
	fmt.Printf("\nFound %d toolchain(s):\n\n", len(toolchains))
	for i, tc := range toolchains {
		fmt.Printf("%d. %s\n", i+1, tc.String())
		fmt.Printf("   Path: %s\n", tc.CompilerPath)
		fmt.Printf("   Version: %s\n", tc.CompilerVersion)
		if len(tc.AbiModifiers) > 0 {
			fmt.Printf("   ABI Modifiers: %v\n", tc.AbiModifiers)
		}
		fmt.Println()
	}

	// Display summary
	fmt.Printf("Summary: %s\n", registry.Summary())

	// Example: Check specific toolchains
	fmt.Println("\nToolchain Availability:")
	if gcc := registry.GetGCC(ctx); gcc != nil {
		fmt.Printf("✓ GCC available: %s\n", gcc.CompilerPath)
	} else {
		fmt.Println("✗ GCC not available")
	}

	if gxx := registry.GetGxx(ctx); gxx != nil {
		fmt.Printf("✓ G++ available: %s\n", gxx.CompilerPath)
	} else {
		fmt.Println("✗ G++ not available")
	}

	if clang := registry.GetClang(ctx); clang != nil {
		fmt.Printf("✓ Clang available: %s\n", clang.CompilerPath)
	} else {
		fmt.Println("✗ Clang not available")
	}

	if clangxx := registry.GetClangxx(ctx); clangxx != nil {
		fmt.Printf("✓ Clang++ available: %s\n", clangxx.CompilerPath)
	} else {
		fmt.Println("✗ Clang++ not available")
	}
}
