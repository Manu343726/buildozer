package main

import (
	"context"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/drivers"
	clangdriver "github.com/Manu343726/buildozer/pkg/drivers/cpp/clang"
	"github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
)

func main() {
	drivers.StandardDriverFlags.Parse(nil)

	driver := &drivers.SimpleDriver{
		DriverName:           "clang",
		DriverVersion:        "14.0.0",
		DriverShort:          "Clang C compiler",
		DriverLong:           "Clang - LLVM C compiler. Compatible with GCC flags.",
		DriverPrefix:         "clang: error:",
		DriverLanguageType:   drivers.Clang,
		DriverSupportsStdlib: false,
		DriverParseCommandLine: func(args []string) interface{} {
			return gcc_common.ParseCommandLine(args)
		},
		DriverCreateJob: func(ctx context.Context, parsed interface{}, runtime *v1.Runtime, workDir string) (*v1.Job, error) {
			return clangdriver.CreateJob(ctx, parsed.(*gcc_common.ParsedArgs), runtime, workDir)
		},
		DriverToolArgsApplier: func(ctx context.Context) drivers.ToolArgsApplier {
			return func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
				flags := gcc_common.ExtractCompilerFlags(toolArgs)
				return gcc_common.ModifyRuntimeIDWithFlags(baseRuntime, flags)
			}
		},
		DriverRuntimeValidator: func(runtime *v1.Runtime) (bool, string) {
			compat := gcc_common.ValidateRuntimeForC(runtime)
			return compat.IsCompatible, compat.Reason
		},
	}

	drivers.ExecuteDriver(driver)
}
