package main

import (
	"context"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/drivers"
	"github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
	gxxdriver "github.com/Manu343726/buildozer/pkg/drivers/cpp/gxx"
)

func main() {
	drivers.StandardDriverFlags.Parse(nil)

	driver := &drivers.SimpleDriver{
		DriverName:           "g++",
		DriverVersion:        "10.2.1",
		DriverShort:          "G++ C++ compiler",
		DriverLong:           "G++ - the GNU Compiler Collection for C++.",
		DriverPrefix:         "g++: error:",
		DriverLanguageType:   drivers.GXX,
		DriverSupportsStdlib: true,
		DriverParseCommandLine: func(args []string) interface{} {
			return gcc_common.ParseCommandLine(args)
		},
		DriverCreateJob: func(ctx context.Context, parsed interface{}, runtime *v1.Runtime, workDir string) (*v1.Job, error) {
			return gxxdriver.CreateJob(ctx, parsed.(*gcc_common.ParsedArgs), runtime, workDir)
		},
		DriverToolArgsApplier: func(ctx context.Context) drivers.ToolArgsApplier {
			return func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
				flags := gcc_common.ExtractCompilerFlags(toolArgs)
				return gcc_common.ModifyRuntimeIDWithFlags(baseRuntime, flags)
			}
		},
		DriverRuntimeValidator: func(runtime *v1.Runtime) (bool, string) {
			compat := gcc_common.ValidateRuntimeForCxx(runtime)
			return compat.IsCompatible, compat.Reason
		},
	}

	drivers.ExecuteDriver(driver)
}
