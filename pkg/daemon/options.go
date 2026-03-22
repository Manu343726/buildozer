package daemon

import (
	"fmt"
)

// Builder provides a fluent interface for constructing a Daemon with custom options.
// This pattern allows for flexible configuration while maintaining reasonable defaults.
type Builder struct {
	config DaemonConfig
}

// NewBuilder creates a new Builder with sensible defaults.
func NewBuilder() *Builder {
	return &Builder{
		config: DaemonConfig{
			Host:              "localhost",
			Port:              6789,
			MaxConcurrentJobs: 4,
			MaxRAMMB:          4096, // 4GB
			EnableMDNS:        false,
		},
	}
}

// Host sets the host to listen on.
func (b *Builder) Host(host string) *Builder {
	b.config.Host = host
	return b
}

// Port sets the port to listen on.
func (b *Builder) Port(port int) *Builder {
	if port <= 0 || port > 65535 {
		panic(fmt.Sprintf("invalid port: %d (must be 1-65535)", port))
	}
	b.config.Port = port
	return b
}

// MaxConcurrentJobs sets the maximum number of concurrent jobs.
func (b *Builder) MaxConcurrentJobs(max int) *Builder {
	if max <= 0 {
		panic(fmt.Sprintf("invalid max concurrent jobs: %d (must be > 0)", max))
	}
	b.config.MaxConcurrentJobs = max
	return b
}

// MaxRAMMB sets the maximum RAM to use (in MB).
func (b *Builder) MaxRAMMB(mb int) *Builder {
	if mb <= 0 {
		panic(fmt.Sprintf("invalid max RAM: %d MB (must be > 0)", mb))
	}
	b.config.MaxRAMMB = mb
	return b
}

// EnableMDNS enables mDNS peer discovery.
func (b *Builder) EnableMDNS(enable bool) *Builder {
	b.config.EnableMDNS = enable
	return b
}

// Build creates and returns a Daemon with the configured settings.
// This creates a fully initialized daemon with all services.
func (b *Builder) Build() (*Daemon, error) {
	return NewDaemon(b.config)
}

// BuildWithConfig creates a new Daemon using an explicit DaemonConfig.
// This is a convenience method for cases where you want to construct a config separately.
func BuildWithConfig(config DaemonConfig) (*Daemon, error) {
	return NewDaemon(config)
}
