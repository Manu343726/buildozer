package clang

import (
	"context"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/driver"
	gcc_common "github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
)

type clangDriver struct{}

// NewDriver returns a driver.Driver implementation for Clang (C compiler).
func NewDriver() driver.Driver { return clangDriver{} }

func (clangDriver) Name() string        { return "clang" }
func (clangDriver) Version() string     { return "14.0.0" }
func (clangDriver) Short() string       { return "Clang C compiler" }
func (clangDriver) Long() string        { return "Clang - LLVM C compiler. Compatible with GCC flags." }
func (clangDriver) ErrorPrefix() string { return "clang: error:" }

func (clangDriver) ValidateArgs(args []string) error {
	cfg := &gcc_common.CLIConfig{Name: "clang", Type: gcc_common.Clang}
	_, err := gcc_common.ValidateAndParseArgs(args, cfg)
	return err
}

func (clangDriver) ParseCommandLine(args []string) interface{} {
	return gcc_common.ParseCommandLine(args)
}

func (clangDriver) CreateJob(ctx context.Context, parsed interface{}, runtime *v1.Runtime, workDir string) (*v1.Job, error) {
	return gcc_common.CreateCppJob(ctx, parsed.(*gcc_common.ParsedArgs), runtime, workDir)
}

func (clangDriver) ApplyToolArgs(_ context.Context, baseRuntime *v1.Runtime, toolArgs []string) (*v1.Runtime, error) {
	flags := gcc_common.ExtractCompilerFlags(toolArgs)
	return gcc_common.ModifyRuntimeWithFlags(baseRuntime, flags)
}

func (clangDriver) ValidateRuntime(runtime *v1.Runtime) (bool, string) {
	compat := gcc_common.ValidateRuntimeForClang(runtime)
	return compat.IsCompatible, compat.Reason
}
