package gcc

import (
	"context"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/driver"
	gcc_common "github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
)

type gccDriver struct{}

// NewDriver returns a driver.Driver implementation for GCC (C compiler).
func NewDriver() driver.Driver { return gccDriver{} }

func (gccDriver) Name() string        { return "gcc" }
func (gccDriver) Version() string     { return "10.2.1" }
func (gccDriver) Short() string       { return "GCC C compiler" }
func (gccDriver) Long() string        { return "GCC - the GNU Compiler Collection for C." }
func (gccDriver) ErrorPrefix() string { return "gcc: error:" }

func (gccDriver) ValidateArgs(args []string) error {
	cfg := &gcc_common.CLIConfig{Name: "gcc", Type: gcc_common.GCC}
	_, err := gcc_common.ValidateAndParseArgs(args, cfg)
	return err
}

func (gccDriver) ParseCommandLine(args []string) interface{} {
	return gcc_common.ParseCommandLine(args)
}

func (gccDriver) CreateJob(ctx context.Context, parsed interface{}, runtime *v1.Runtime, workDir string) (*v1.Job, error) {
	return gcc_common.CreateCppJob(ctx, parsed.(*gcc_common.ParsedArgs), runtime, workDir)
}

func (gccDriver) ApplyToolArgs(_ context.Context, baseRuntime *v1.Runtime, toolArgs []string) (*v1.Runtime, error) {
	flags := gcc_common.ExtractCompilerFlags(toolArgs)
	return gcc_common.ModifyRuntimeWithFlags(baseRuntime, flags)
}

func (gccDriver) ValidateRuntime(runtime *v1.Runtime) (bool, string) {
	compat := gcc_common.ValidateRuntimeForC(runtime)
	return compat.IsCompatible, compat.Reason
}
