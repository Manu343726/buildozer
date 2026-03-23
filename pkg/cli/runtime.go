package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/pkg/config"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// RuntimeCommands provides command-level implementations for runtime CLI operations.
type RuntimeCommands struct {
	*logging.Logger // Embedded logger for hierarchical logging
}

// NewRuntimeCommands creates a new RuntimeCommands handler.
func NewRuntimeCommands(cfg *config.Config) (*RuntimeCommands, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration not initialized")
	}

	return &RuntimeCommands{
		Logger: Log().Child("RuntimeCommands"),
	}, nil
}

// List queries the daemon for runtimes (and optionally peers if localOnly is false)
func (rc *RuntimeCommands) List(localOnly bool) error {
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("configuration not initialized")
	}

	daemonURL := fmt.Sprintf("http://%s:%d", cfg.Daemon.Host, cfg.Daemon.Port)
	client := protov1connect.NewRuntimeServiceClient(http.DefaultClient, daemonURL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query the daemon for available runtimes
	resp, err := client.ListRuntimes(ctx, connect.NewRequest(&v1.ListRuntimesRequest{
		LocalOnly: localOnly,
	}))
	if err != nil {
		return fmt.Errorf("failed to query daemon runtimes: %w", err)
	}

	runtimes := resp.Msg.Runtimes
	notes := resp.Msg.DetectionNotes

	if len(runtimes) == 0 {
		fmt.Println("No runtimes available")
		if notes != "" {
			fmt.Printf("Note: %s\n", notes)
		}
		return nil
	}

	fmt.Printf("Available Runtimes:\n\n")

	for i, rt := range runtimes {
		displayRuntime(i+1, rt)
	}

	// Display notes
	if notes != "" {
		fmt.Printf("Notes: %s\n\n", notes)
	}

	// Display summary statistics
	displayRuntimeSummary(runtimes)

	return nil
}

// Info queries the daemon for a specific runtime's details
func (rc *RuntimeCommands) Info(runtimeID string) error {
	if runtimeID == "" {
		return fmt.Errorf("runtime ID is required")
	}

	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("configuration not initialized")
	}

	daemonURL := fmt.Sprintf("http://%s:%d", cfg.Daemon.Host, cfg.Daemon.Port)
	client := protov1connect.NewRuntimeServiceClient(http.DefaultClient, daemonURL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query the daemon for the specific runtime
	resp, err := client.GetRuntime(ctx, connect.NewRequest(&v1.GetRuntimeRequest{
		RuntimeId: runtimeID,
	}))
	if err != nil {
		return fmt.Errorf("failed to query daemon for runtime: %w", err)
	}

	if resp.Msg.Error != nil && *resp.Msg.Error != "" {
		fmt.Printf("Error: %s\n", *resp.Msg.Error)
		return nil
	}

	if resp.Msg.Runtime == nil {
		fmt.Printf("Runtime not found: %s\n", runtimeID)
		return nil
	}

	fmt.Printf("Runtime Information for: %s\n", runtimeID)
	fmt.Println()
	displayRuntime(1, resp.Msg.Runtime)

	return nil
}

// displayRuntime shows details about a single runtime
func displayRuntime(num int, rt *v1.Runtime) {
	fmt.Printf("%d. %s\n", num, rt.Id)

	// Display toolchain-specific details
	switch toolchain := rt.Toolchain.(type) {
	case *v1.Runtime_Cpp:
		cpp := toolchain.Cpp
		if cpp != nil {
			lang := "Unknown"
			switch cpp.Language {
			case v1.CppLanguage_CPP_LANGUAGE_C:
				lang = "C"
			case v1.CppLanguage_CPP_LANGUAGE_CPP:
				lang = "C++"
			}

			compilerName := "Unknown"
			switch cpp.Compiler {
			case v1.CppCompiler_CPP_COMPILER_GCC:
				compilerName = "GCC"
			case v1.CppCompiler_CPP_COMPILER_CLANG:
				compilerName = "Clang"
			}

			fmt.Printf("   Type:        C/C++\n")
			fmt.Printf("   Language:    %s\n", lang)
			fmt.Printf("   Compiler:    %s", compilerName)
			if cpp.CompilerVersion != nil {
				fmt.Printf(" %s", formatVersion(cpp.CompilerVersion))
			}
			fmt.Printf("\n")

			archName := "Unknown"
			switch cpp.Architecture {
			case v1.CpuArchitecture_CPU_ARCHITECTURE_X86_64:
				archName = "x86_64"
			case v1.CpuArchitecture_CPU_ARCHITECTURE_AARCH64:
				archName = "aarch64"
			case v1.CpuArchitecture_CPU_ARCHITECTURE_ARM:
				archName = "arm"
			}
			fmt.Printf("   Architecture: %s\n", archName)

			cruntimeName := "Unknown"
			switch cpp.CRuntime {
			case v1.CRuntime_C_RUNTIME_GLIBC:
				cruntimeName = "glibc"
			case v1.CRuntime_C_RUNTIME_MUSL:
				cruntimeName = "musl"
			}
			fmt.Printf("   C Runtime:   %s", cruntimeName)
			if cpp.CRuntimeVersion != nil {
				fmt.Printf(" %s", formatVersion(cpp.CRuntimeVersion))
			}
			fmt.Printf("\n")

			if cpp.CppStdlib != v1.CppStdlib_CPP_STDLIB_UNSPECIFIED {
				stdlibName := "Unknown"
				switch cpp.CppStdlib {
				case v1.CppStdlib_CPP_STDLIB_LIBSTDCXX:
					stdlibName = "libstdc++"
				case v1.CppStdlib_CPP_STDLIB_LIBCXX:
					stdlibName = "libc++"
				}
				fmt.Printf("   C++ Stdlib:  %s\n", stdlibName)
			}
		}
	case *v1.Runtime_Go:
		fmt.Printf("   Type:        Go\n")
		if toolchain.Go != nil {
			fmt.Printf("   Go Version:  %s\n", formatVersion(toolchain.Go.GoVersion))
			fmt.Printf("   GOOS:        %s\n", toolchain.Go.Goos)
			fmt.Printf("   GOARCH:      %s\n", toolchain.Go.Goarch)
		}
	case *v1.Runtime_Rust:
		fmt.Printf("   Type:        Rust\n")
		if toolchain.Rust != nil {
			fmt.Printf("   Rust Version: %s\n", formatVersion(toolchain.Rust.RustVersion))
			fmt.Printf("   Target:      %s\n", toolchain.Rust.TargetTriple)
		}
	}

	fmt.Printf("   Native:      %v\n", rt.IsNative)

	if rt.Description != nil && *rt.Description != "" {
		fmt.Printf("   Description: %s\n", *rt.Description)
	}

	fmt.Println()
}

// formatVersion formats a proto Version message
func formatVersion(v *v1.Version) string {
	if v == nil {
		return ""
	}
	parts := []interface{}{v.Major}
	if v.Minor != nil && *v.Minor > 0 || v.Patch != nil && *v.Patch > 0 {
		if v.Minor != nil {
			parts = append(parts, ".", *v.Minor)
		}
	}
	if v.Patch != nil && *v.Patch > 0 {
		if v.Minor == nil {
			parts = append(parts, ".0")
		}
		parts = append(parts, ".", *v.Patch)
	}
	return fmt.Sprint(parts...)
}

// displayRuntimeSummary shows statistics about detected runtimes
func displayRuntimeSummary(runtimes []*v1.Runtime) {
	typeCount := make(map[string]int)
	archCount := make(map[string]int)

	for _, rt := range runtimes {
		// Count by type
		switch rt.Toolchain.(type) {
		case *v1.Runtime_Cpp:
			typeCount["C/C++"]++
			if cpp := rt.GetCpp(); cpp != nil {
				archName := "Unknown"
				switch cpp.Architecture {
				case v1.CpuArchitecture_CPU_ARCHITECTURE_X86_64:
					archName = "x86_64"
				case v1.CpuArchitecture_CPU_ARCHITECTURE_AARCH64:
					archName = "aarch64"
				case v1.CpuArchitecture_CPU_ARCHITECTURE_ARM:
					archName = "arm"
				}
				archCount[archName]++
			}
		case *v1.Runtime_Go:
			typeCount["Go"]++
		case *v1.Runtime_Rust:
			typeCount["Rust"]++
		}
	}

	fmt.Println("Summary:")
	fmt.Printf("Total: %d runtime(s)\n", len(runtimes))
	if len(typeCount) > 0 {
		fmt.Println("  By Type:")
		for typ, count := range typeCount {
			fmt.Printf("    %s: %d\n", typ, count)
		}
	}
	if len(archCount) > 0 {
		fmt.Println("  By Architecture (C/C++):")
		for arch, count := range archCount {
			fmt.Printf("    %s: %d\n", arch, count)
		}
	}
}
