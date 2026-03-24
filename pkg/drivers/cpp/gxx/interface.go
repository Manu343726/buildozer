package gxx

import (
	"context"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/driver"
	gcc_common "github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
)

type gxxDriver struct{}

// NewDriver returns a driver.Driver implementation for G++ (C++ compiler).
func NewDriver() driver.Driver { return gxxDriver{} }

func (gxxDriver) Name() string        { return "g++" }
func (gxxDriver) Version() string     { return "10.2.1" }
func (gxxDriver) Short() string       { return "G++ C++ compiler" }
func (gxxDriver) Long() string        { return "G++ - the GNU Compiler Collection for C++." }
func (gxxDriver) ErrorPrefix() string { return "g++: error:" }

func (gxxDriver) ValidateArgs(args []string) error {
	cfg := &gcc_common.CLIConfig{Name: "g++", Type: gcc_common.GXX, SupportsStdlib: true}
	_, err := gcc_common.ValidateAndParseArgs(args, cfg)
	return err
}

func (gxxDriver) ParseCommandLine(args []string) interface{} {
	return gcc_common.ParseCommandLine(args)
}

func (gxxDriver) CreateJob(ctx context.Context, parsed interface{}, runtime *v1.Runtime, workDir string) (*v1.Job, error) {
	return gcc_common.CreateCppJob(ctx, parsed.(*gcc_common.ParsedArgs), runtime, workDir)
}

func (gxxDriver) ApplyToolArgs(_ context.Context, baseRuntime *v1.Runtime, toolArgs []string) (*v1.Runtime, error) {
	flags := gcc_common.ExtractCompilerFlags(toolArgs)
	return gcc_common.ModifyRuntimeWithFlags(baseRuntime, flags)
}

func (gxxDriver) ValidateRuntime(runtime *v1.Runtime) (bool, string) {
	compat := gcc_common.ValidateRuntimeForCxx(runtime)
	return compat.IsCompatible, compat.Reason
}
