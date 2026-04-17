package ar_common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCommandLine_EmptyArgs(t *testing.T) {
	parsed := ParseCommandLine([]string{})
	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeWrite, parsed.Mode, "empty args should default to ModeWrite")
	assert.Empty(t, parsed.Flags, "flags should be empty")
	assert.Empty(t, parsed.InputFiles, "input files should be empty")
	assert.Empty(t, parsed.OutputFile, "output file should be empty")
}

func TestParseCommandLine_SimpleReplace(t *testing.T) {
	// ar r archive.a file1.o file2.o
	args := []string{"r", "archive.a", "file1.o", "file2.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeWrite, parsed.Mode, "r command should be ModeWrite")
	assert.ElementsMatch(t, []string{"r"}, parsed.Flags, "flags should contain 'r'")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o", "file2.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_ReplaceWithDash(t *testing.T) {
	// ar -r archive.a file1.o
	args := []string{"-r", "archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeWrite, parsed.Mode, "mode should be ModeWrite")
	assert.ElementsMatch(t, []string{"r"}, parsed.Flags, "flags should contain 'r'")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_MultipleModifiers(t *testing.T) {
	// ar rcv archive.a file1.o file2.o
	args := []string{"rcv", "archive.a", "file1.o", "file2.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	// When r (write), c (create), and v (verbose) are combined, v is processed last
	// so the mode becomes ModeVerbose. This is the current behavior.
	assert.Equal(t, ModeVerbose, parsed.Mode, "v modifier sets ModeVerbose when processed last")
	assert.Len(t, parsed.Flags, 3, "should have 3 flags: r, c, v")
	assert.ElementsMatch(t, []string{"r", "c", "v"}, parsed.Flags, "flags should be r, c, v")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o", "file2.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_QuickAppend(t *testing.T) {
	// ar q archive.a file1.o
	args := []string{"q", "archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeQuick, parsed.Mode, "q command should be ModeQuick")
	assert.ElementsMatch(t, []string{"q"}, parsed.Flags, "flags should contain 'q'")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_UpdateMode(t *testing.T) {
	// ar ru archive.a file1.o
	args := []string{"ru", "archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeUpdate, parsed.Mode, "u modifier should set ModeUpdate")
	assert.ElementsMatch(t, []string{"r", "u"}, parsed.Flags, "flags should contain r and u")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_CreateMode(t *testing.T) {
	// ar rc archive.a file1.o
	args := []string{"rc", "archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeCreate, parsed.Mode, "c modifier should set ModeCreate")
	assert.ElementsMatch(t, []string{"r", "c"}, parsed.Flags, "flags should contain r and c")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_VerboseMode(t *testing.T) {
	// ar rv archive.a file1.o
	args := []string{"rv", "archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeVerbose, parsed.Mode, "v modifier should set ModeVerbose")
	assert.ElementsMatch(t, []string{"r", "v"}, parsed.Flags, "flags should contain r and v")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_ListCommand(t *testing.T) {
	// ar t archive.a
	args := []string{"t", "archive.a"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.ElementsMatch(t, []string{"t"}, parsed.Flags, "flags should contain t")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.Empty(t, parsed.InputFiles, "no input files needed for listing")
}

func TestParseCommandLine_ExtractCommand(t *testing.T) {
	// ar x archive.a file1.o
	args := []string{"x", "archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.ElementsMatch(t, []string{"x"}, parsed.Flags, "flags should contain x")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_DeleteCommand(t *testing.T) {
	// ar d archive.a file1.o
	args := []string{"d", "archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.ElementsMatch(t, []string{"d"}, parsed.Flags, "flags should contain d")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_MoveCommand(t *testing.T) {
	// ar m archive.a file1.o
	args := []string{"m", "archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.ElementsMatch(t, []string{"m"}, parsed.Flags, "flags should contain m")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_PrintCommand(t *testing.T) {
	// ar p archive.a
	args := []string{"p", "archive.a"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.ElementsMatch(t, []string{"p"}, parsed.Flags, "flags should contain p")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.Empty(t, parsed.InputFiles, "no input files needed for print")
}

func TestParseCommandLine_LongOptionTarget(t *testing.T) {
	// ar --target=elf64-x86-64 r archive.a file1.o
	args := []string{"--target=elf64-x86-64", "r", "archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeWrite, parsed.Mode, "r command should set ModeWrite")
	assert.ElementsMatch(t, []string{"r"}, parsed.Flags, "flags should contain r")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_LongOptionWithSpace(t *testing.T) {
	// ar --target elf64-x86-64 r archive.a file1.o
	args := []string{"--target", "elf64-x86-64", "r", "archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeWrite, parsed.Mode, "r command should set ModeWrite")
	assert.ElementsMatch(t, []string{"r"}, parsed.Flags, "flags should contain r")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_MultipleInputFiles(t *testing.T) {
	// ar r archive.a file1.o file2.o file3.o file4.o
	args := []string{"r", "archive.a", "file1.o", "file2.o", "file3.o", "file4.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.Len(t, parsed.InputFiles, 4, "should have 4 input files")
	assert.ElementsMatch(t, []string{"file1.o", "file2.o", "file3.o", "file4.o"}, parsed.InputFiles, "all input files should match")
}

func TestParseCommandLine_OutputFileWithPath(t *testing.T) {
	// ar r path/to/archive.a file1.o
	args := []string{"r", "path/to/archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, "path/to/archive.a", parsed.OutputFile, "output file should preserve path")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_InputFilesWithPath(t *testing.T) {
	// ar r archive.a path/to/file1.o ../file2.o
	args := []string{"r", "archive.a", "path/to/file1.o", "../file2.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, "archive.a", parsed.OutputFile, "output file should match")
	assert.ElementsMatch(t, []string{"path/to/file1.o", "../file2.o"}, parsed.InputFiles, "input files should preserve paths")
}

func TestParseCommandLine_ArchiveFileAfterOptions(t *testing.T) {
	// ar --output=. -r archive.a file1.o
	args := []string{"--output=.", "-r", "archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file regardless of options")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_FileStartingWithDash(t *testing.T) {
	// When a file legitimately starts with dash, it comes after archive-file
	// ar r archive.a -file1.o
	// In this case, -file1.o should be treated as an input file, not a flag
	args := []string{"r", "archive.a", "-file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"-file1.o"}, parsed.InputFiles, "input file starting with dash should be preserved")
}

func TestParseCommandLine_ComplexFlags(t *testing.T) {
	// ar rscuD archive.a file1.o file2.o
	args := []string{"rscuD", "archive.a", "file1.o", "file2.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Len(t, parsed.Flags, 5, "should have 5 flags")
	assert.ElementsMatch(t, []string{"r", "s", "c", "u", "D"}, parsed.Flags, "all flags should be extracted")
	assert.Equal(t, ModeUpdate, parsed.Mode, "u modifier should determine mode (ModeUpdate)")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o", "file2.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_CaseSensitiveModifiers(t *testing.T) {
	// ar rST archive.a file1.o
	args := []string{"rST", "archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.ElementsMatch(t, []string{"r", "S", "T"}, parsed.Flags, "uppercase modifiers should be preserved")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_PluginOption(t *testing.T) {
	// ar --plugin llvm-ar-18 r archive.a file1.o
	args := []string{"--plugin", "llvm-ar-18", "r", "archive.a", "file1.o"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.ElementsMatch(t, []string{"r"}, parsed.Flags, "flags should contain r")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_ResponseFile(t *testing.T) {
	// ar r archive.a @filelist.txt
	args := []string{"r", "archive.a", "@filelist.txt"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.ElementsMatch(t, []string{"@filelist.txt"}, parsed.InputFiles, "response file should be in input files")
}

// Validation Tests
func TestValidateAndParseArgs_MissingArgs(t *testing.T) {
	_, err := ValidateAndParseArgs([]string{})
	require.Error(t, err, "should error on empty args")
	assert.Equal(t, ErrMissingArg, err, "should return ErrMissingArg")
}

func TestValidateAndParseArgs_SingleArg(t *testing.T) {
	_, err := ValidateAndParseArgs([]string{"r"})
	require.Error(t, err, "should error on single arg")
	assert.Equal(t, ErrMissingArg, err, "should return ErrMissingArg")
}

func TestValidateAndParseArgs_MissingOutputFile(t *testing.T) {
	_, err := ValidateAndParseArgs([]string{"r", "--target=elf64-x86-64"})
	require.Error(t, err, "should error when no archive file specified")
	assert.Equal(t, ErrNoOutputFile, err, "should return ErrNoOutputFile")
}

func TestValidateAndParseArgs_ValidArgs(t *testing.T) {
	parsed, err := ValidateAndParseArgs([]string{"r", "archive.a", "file1.o"})
	require.NoError(t, err, "should not error on valid args")
	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, "archive.a", parsed.OutputFile, "output file should match")
	assert.ElementsMatch(t, []string{"file1.o"}, parsed.InputFiles, "input files should match")
}

func TestParseCommandLine_OnlyCommandAndArchive(t *testing.T) {
	// ar t archive.a (list contents without errors)
	args := []string{"t", "archive.a"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, "archive.a", parsed.OutputFile, "archive.a should be output file")
	assert.Empty(t, parsed.InputFiles, "no input files needed for certain operations")
}

func TestParseCommandLine_DashModifierVariations(t *testing.T) {
	testCases := []struct {
		name        string
		args        []string
		expectedCmd string
		expectedOut string
	}{
		{"dash-r", []string{"-r", "arch.a", "file.o"}, "r", "arch.a"},
		{"dash-rcv", []string{"-rcv", "arch.a", "file.o"}, "r", "arch.a"},
		{"nodash-r", []string{"r", "arch.a", "file.o"}, "r", "arch.a"},
		{"nodash-rcv", []string{"rcv", "arch.a", "file.o"}, "r", "arch.a"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed := ParseCommandLine(tc.args)
			require.NotNil(t, parsed, "parsed result should not be nil")
			assert.Equal(t, tc.expectedOut, parsed.OutputFile, "output file should match")
			assert.Contains(t, parsed.Flags, tc.expectedCmd, "flags should contain command")
		})
	}
}

func TestIsCommandString(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
		desc     string
	}{
		{"r", true, "single command"},
		{"-r", true, "command with dash"},
		{"rcv", true, "command with modifiers"},
		{"-rcv", true, "command with modifiers and dash"},
		{"ruD", true, "command with uppercase modifiers"},
		{"", false, "empty string"},
		{"archive.a", false, "file path"},
		{"path/to/file.o", false, "file path with slash"},
		{"-file.o", false, "file starting with dash (not command)"},
		{"--target", false, "long option"},
		{"xyz", false, "invalid command"},
		{"rxyz", false, "invalid modifiers"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := isCommandString(tc.input)
			assert.Equal(t, tc.expected, result, "isCommandString result should match expected for: %s", tc.desc)
		})
	}
}

func TestExtractCommandAndModifiers(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
		desc     string
	}{
		{"r", []string{"r"}, "single command"},
		{"-r", []string{"r"}, "command with dash"},
		{"rcv", []string{"r", "c", "v"}, "command with modifiers"},
		{"-rcv", []string{"r", "c", "v"}, "command with modifiers and dash"},
		{"ruDfS", []string{"r", "u", "D", "f", "S"}, "multiple modifiers"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := extractCommandAndModifiers(tc.input)
			assert.ElementsMatch(t, tc.expected, result, "extracted flags should match for: %s", tc.desc)
		})
	}
}

func TestUpdateModeFromFlags(t *testing.T) {
	testCases := []struct {
		flags        []string
		expectedMode ArchiveMode
		desc         string
	}{
		{[]string{"r"}, ModeWrite, "r flag"},
		{[]string{"r", "u"}, ModeUpdate, "r and u flags"},
		{[]string{"r", "c"}, ModeCreate, "r and c flags"},
		{[]string{"r", "v"}, ModeVerbose, "r and v flags"},
		{[]string{"q"}, ModeQuick, "q flag"},
		{[]string{}, ModeWrite, "no flags"},
		{[]string{"q", "u"}, ModeUpdate, "q and u flags (u wins)"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			parsed := &ParsedArgs{Flags: tc.flags}
			updateModeFromFlags(parsed)
			assert.Equal(t, tc.expectedMode, parsed.Mode, "mode should match for: %s", tc.desc)
		})
	}
}
