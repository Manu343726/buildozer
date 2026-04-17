package sandbox

import (
	"fmt"

	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtime"
)

// Builder composes multiple sandboxes with a fluent API.
// The logger is provided once during builder creation and passed to all sandboxes.
type Builder struct {
	params    SandboxParams
	factories []SandboxFactory
}

// NewBuilder creates a new sandbox pipeline builder with a logger.
func NewBuilder(logger *logging.Logger) *Builder {
	if logger == nil {
		logger = logging.Log().Child("sandbox-builder")
	}
	return &Builder{
		params: SandboxParams{Logger: logger},
	}
}

// With adds a sandbox factory to the pipeline.
//
// Example:
//
//	pipeline := sandbox.NewBuilder(logger).
//	    With(EmbedInputs()).
//	    With(TempDir()).
//	    With(Workdir("/tmp")).
//	    Build()
func (b *Builder) With(factory SandboxFactory) *Builder {
	b.factories = append(b.factories, factory)
	return b
}

// Build composes all added factories into a single SandboxFunc.
// The sandboxes are applied left-to-right during wrapping.
func (b *Builder) Build() SandboxFunc {
	factories := make([]SandboxFactory, len(b.factories))
	copy(factories, b.factories)
	params := b.params

	return func(rt runtime.Runtime) (runtime.Runtime, error) {
		wrapped := rt
		for _, factory := range factories {
			sandbox := factory(params)
			var err error
			wrapped, err = sandbox(wrapped)
			if err != nil {
				return nil, fmt.Errorf("sandbox failed: %w", err)
			}
		}
		return wrapped, nil
	}
}
