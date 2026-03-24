package clangxx

import (
	"context"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/driver"
	gcc_common "github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
)

type clangxxDriver struct{}

// NewDriver returns a driver.Driver implementation for Clang++ (C++ compiler).
func NewDriver() driver.Driver { return clangxxDriver{} }

func (clangxxDriver) Name() string        { return "clang++" }
func (clangxxDriver) Version() string     { return "14.0.0" }
func (clangxxDriver) Short() string       { return "Clang++ C++ compiler" }
func (clangxxDriver) Long() string        { return "Clang++ - LLVM C++ compiler. Compatible with G++ flags." }
func (clangxxDriver) ErrorPrefix() string { return "clang++: error:" }

func (clangxxDriver) ValidateArgs(args []string) error {
	cfg := &gcc_common.CLIConfig{Name: "clang++", Type: gcc_common.ClangCxx, SupportsStdlib: true}
	_, err := gcc_common.ValidateAndParseArgs(args, cfg)
	return err
}

func (clangxxDriver) ParseCommandLine(args []string) interface{} {
	return gcc_common.ParseCommandLine(args)
}

func (clangxxDriver) CreateJob(ctx context.Context, parsed interface{}, runtime *v1.Runtime, workDir string) (*v1.Job, error) {
	return gcc_common.CreateCppJob(ctx, parsed.(*gcc_common.ParsedArgs), runtime, workDir)
}

func (clangxxDriver) ApplyToolArgs(_ context.Context, baseRuntime *v1.Runtime, toolArgs []string) (*v1.Runtime, error) {
	flags := gcc_common.ExtractCompilerFlags(toolArgs)
	return gcc_common.ModifyRuntimeWithFlags(baseRuntime, flags)
}

func (clangxxDriver) ValidateRuntime(runtime *v1.Runtime) (bool, string) {
	compat := gcc_common.ValidateRuntimeForClangxx(runtime)
	return compat.IsCompatible, compat.Reason
}
