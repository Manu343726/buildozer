package sandbox

import (
	"fmt"

	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtime"
)

// Pipe returns a SandboxFunc that applies multiple sandbox factories in sequence.
// All factories are called with the provided params to create SandboxFuncs,
// then applied to the runtime left-to-right.
//
// Usage:
//
//	params := sandbox.SandboxParams{Logger: logger}
//	pipeline := sandbox.Pipe(params,
//	    EmbedInputs(),
//	    TempDir(),
//	    Workdir("/tmp"),
//	)
//	wrapped, err := pipeline(rt)
func Pipe(params SandboxParams, factories ...SandboxFactory) SandboxFunc {
	if params.Logger == nil {
		params.Logger = logging.Log().Child("pipe")
	}

	return func(rt runtime.Runtime) (runtime.Runtime, error) {
		return Apply(rt, params, factories...)
	}
}

// Apply applies multiple sandbox factories to a runtime in sequence.
// Each factory is called with the provided params to create a SandboxFunc,
// then applied to the runtime left-to-right.
//
// Usage:
//
//	params := sandbox.SandboxParams{Logger: logger}
//	wrapped, err := Apply(rt, params,
//	    EmbedInputs(),
//	    TempDir(),
//	    Workdir("/tmp"),
//	)
func Apply(rt runtime.Runtime, params SandboxParams, factories ...SandboxFactory) (runtime.Runtime, error) {
	if params.Logger == nil {
		params.Logger = logging.Log().Child("apply")
	}

	wrapped := rt
	for i, factory := range factories {
		sandbox := factory(params)
		var err error
		wrapped, err = sandbox(wrapped)
		if err != nil {
			return nil, fmt.Errorf("sandbox[%d] failed: %w", i, err)
		}
	}
	return wrapped, nil
}

// MustApply applies multiple sandbox factories to a runtime in sequence.
// It returns only the final sandboxed runtime and panics if any sandbox fails.
// The panic is logged using the provided logger before panicking.
//
// Usage:
//
//	params := sandbox.SandboxParams{Logger: logger}
//	wrapped := MustApply(rt, params,
//	    EmbedInputs(),
//	    TempDir(),
//	    Workdir("/tmp"),
//	)
func MustApply(rt runtime.Runtime, params SandboxParams, factories ...SandboxFactory) runtime.Runtime {
	if params.Logger == nil {
		params.Logger = logging.Log().Child("must-apply")
	}

	wrapped, err := Apply(rt, params, factories...)
	if err != nil {
		params.Logger.Panicf("sandbox pipeline failed: %v", err)
	}
	return wrapped
}

// MustPipe returns a SandboxFunc that applies multiple sandbox factories as a pipeline.
// All factories are called with the provided params to create SandboxFuncs,
// then applied to the runtime left-to-right. Panics if any sandbox fails.
//
// Usage:
//
//	params := sandbox.SandboxParams{Logger: logger}
//	pipeline := sandbox.MustPipe(params,
//	    EmbedInputs(),
//	    TempDir(),
//	    Workdir("/tmp"),
//	)
//	wrapped := pipeline(rt) // panics on error
func MustPipe(params SandboxParams, factories ...SandboxFactory) SandboxFunc {
	if params.Logger == nil {
		params.Logger = logging.Log().Child("must-pipe")
	}

	return func(rt runtime.Runtime) (runtime.Runtime, error) {
		return MustApply(rt, params, factories...), nil
	}
}
