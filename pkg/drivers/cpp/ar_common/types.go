package ar_common

import (
"fmt"
)

// ParsedArgs represents the parsed result of an ar command line invocation
type ParsedArgs struct {
	// Operation flags (r, u, c, v, etc.)
	Flags []string

	// Input files to archive (object files, libraries)
	InputFiles []string

	// Output archive file path
	OutputFile string

	// Original command line args
	OriginalArgs []string

	// Determines the archive operation mode
	Mode ArchiveMode
}

// ArchiveMode represents the type of archive operation
type ArchiveMode int

const (
// ModeWrite - r flag: replace or insert files
ModeWrite ArchiveMode = iota
// ModeUpdate - u flag: replace or insert with update semantics
ModeUpdate
// ModeCreate - c flag: create without printing warnings
ModeCreate
// ModeVerbose - v flag: verbosity
ModeVerbose
// ModeQuick - q flag: append without repositioning existing members
ModeQuick
)

// ArchiveOperation represents the planned archive operation
type ArchiveOperation struct {
	// Output archive file path
	OutputFile string

	// Input files to add to archive
	InputFiles []string

	// Archive flags
	Flags []string

	// Operation mode
	Mode ArchiveMode
}

// String returns the string representation of the archive mode
func (m ArchiveMode) String() string {
	switch m {
	case ModeWrite:
		return "write"
	case ModeUpdate:
		return "update"
	case ModeCreate:
		return "create"
	case ModeVerbose:
		return "verbose"
	case ModeQuick:
		return "quick"
	default:
		return "unknown"
	}
}

// ModeString converts an ArchiveMode to a human-readable string
func ModeString(mode ArchiveMode) string {
	return mode.String()
}

// Error types for ar_common package
var (
ErrNoOutputFile = fmt.Errorf("no output file specified")
ErrNoInputFiles = fmt.Errorf("no input files specified")
ErrInvalidFlags = fmt.Errorf("invalid ar flags")
ErrMissingArg   = fmt.Errorf("missing required argument")
)
