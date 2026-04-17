package ar_common

import (
	"strings"
)

// ParseCommandLine parses raw ar command line arguments into ParsedArgs.
// Follows the GNU ar command line syntax:
//
//	ar [emulation options] [-]{dmpqrstx}[abcDfilMNoOPsSTuvV] [--plugin <name>]
//	   [member-name] [count] archive-file file...
//
// The key insight: archive-file is a positional argument that comes AFTER the
// command/modifier flags, and all files after archive-file are input files.
// Options can appear before the command.
func ParseCommandLine(args []string) *ParsedArgs {
	if len(args) == 0 {
		return &ParsedArgs{
			OriginalArgs: args,
			Mode:         ModeWrite,
		}
	}

	parsed := &ParsedArgs{
		OriginalArgs: args,
		Mode:         ModeWrite,
		Flags:        []string{},
		InputFiles:   []string{},
		OutputFile:   "",
	}

	idx := 0

	// Step 1: Find the command string (skip over leading options/emulation flags)
	// The command string is the first argument that looks like a valid ar command
	for idx < len(args) {
		arg := args[idx]

		// Check if this looks like command+modifiers (not a file path or option)
		if isCommandString(arg) {
			flags := extractCommandAndModifiers(arg)
			parsed.Flags = flags
			idx++
			break
		}

		// Skip options and their arguments
		if skipOption(arg, args, &idx) {
			continue
		}

		// If we get here, it's not a recognized option and not a command
		// This shouldn't happen in well-formed ar commands, but treat it as archive-file
		break
	}

	// Step 2: Skip any command-specific options/parameters that appear after the command
	// These are handled by ar itself, we just need to find the archive-file
	for idx < len(args) {
		arg := args[idx]

		// Skip options and their arguments
		if skipOption(arg, args, &idx) {
			continue
		}

		// This should be the archive-file (first non-option after command)
		break
	}

	// Step 3: Extract archive-file and input files
	if idx < len(args) {
		parsed.OutputFile = args[idx]
		idx++
	}

	// All remaining arguments are input files
	parsed.InputFiles = args[idx:]

	// Step 4: Determine archive mode based on flags
	updateModeFromFlags(parsed)

	return parsed
}

// skipOption checks if current arg is an option and advances idx accordingly
// Returns true if an option was skipped, false otherwise
func skipOption(arg string, args []string, idx *int) bool {
	// Long options with = syntax (these don't consume next arg)
	if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
		*idx++
		return true
	}

	// Long options that take an argument
	if arg == "--plugin" || arg == "--target" || arg == "--output" {
		*idx++ // skip option name
		if *idx < len(args) {
			*idx++ // skip option value
		}
		return true
	}

	// Response file syntax @<file>
	if strings.HasPrefix(arg, "@") {
		*idx++
		return true
	}

	// Any other double-dash option (don't consume next arg)
	if strings.HasPrefix(arg, "--") {
		*idx++
		return true
	}

	// Any other single-dash option that's not a command
	if strings.HasPrefix(arg, "-") && !isCommandString(arg) {
		*idx++
		return true
	}

	return false
}

// isCommandString checks if a string appears to be ar command+modifiers
// rather than a file path. This prevents treating file paths as flags.
func isCommandString(s string) bool {
	if s == "" {
		return false
	}

	// Remove leading dash if present
	str := s
	if strings.HasPrefix(str, "-") {
		str = str[1:]
	}

	if str == "" {
		return false
	}

	// First character must be a valid command: d, m, p, q, r, s, t, x
	command := rune(str[0])
	switch command {
	case 'd', 'D', 'm', 'M', 'p', 'P', 'q', 'Q', 'r', 'R', 's', 'S', 't', 'T', 'x', 'X':
		// Valid command character
	default:
		return false
	}

	// Remaining characters (if any) should be valid modifiers
	// Valid modifiers: a, b, c, D, f, i, l, M, N, o, O, P, s, S, T, u, U, v, V
	for _, ch := range str[1:] {
		switch ch {
		case 'a', 'A', 'b', 'B', 'c', 'C', 'D', 'f', 'F', 'i', 'I', 'l', 'L',
			'M', 'N', 'o', 'O', 'P', 's', 'S', 'T', 'u', 'U', 'v', 'V':
			// Valid modifier
		default:
			// If we find a character that's not a valid modifier, this isn't
			// a command string (could be a file path)
			return false
		}
	}

	return true
}

// extractCommandAndModifiers splits command+modifiers string into individual flags
func extractCommandAndModifiers(s string) []string {
	flags := []string{}

	str := s
	// Remove leading dash if present
	if strings.HasPrefix(str, "-") {
		str = str[1:]
	}

	// Each character becomes a flag
	for _, ch := range str {
		flags = append(flags, string(ch))
	}

	return flags
}

// updateModeFromFlags determines the archive mode based on parsed flags
func updateModeFromFlags(parsed *ParsedArgs) {
	// Default mode is ModeWrite
	parsed.Mode = ModeWrite

	for _, flag := range parsed.Flags {
		f := strings.ToLower(flag)
		switch f {
		case "r":
			parsed.Mode = ModeWrite
		case "u":
			parsed.Mode = ModeUpdate
		case "c":
			parsed.Mode = ModeCreate
		case "v":
			parsed.Mode = ModeVerbose
		case "q":
			parsed.Mode = ModeQuick
		}
	}
}

// ValidateAndParseArgs validates raw arguments and returns parsed result or error
func ValidateAndParseArgs(args []string) (*ParsedArgs, error) {
	if len(args) < 2 {
		return nil, ErrMissingArg
	}

	parsed := ParseCommandLine(args)

	if parsed.OutputFile == "" {
		return nil, ErrNoOutputFile
	}

	// Note: Some ar commands (like 't' for listing) don't require input files
	// So we'll be more lenient here and not enforce input files
	// if len(parsed.InputFiles) == 0 {
	// 	return nil, ErrNoInputFiles
	// }

	return parsed, nil
}
