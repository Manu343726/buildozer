package main

import (
	"testing"
)

func TestParseCommandLineCompileOnly(t *testing.T) {
	args := []string{"test.c", "-c", "-o", "test.o"}
	parsed := ParseCommandLine(args)

	if parsed.Mode != ModeCompileOnly {
		t.Errorf("expected ModeCompileOnly, got %v", parsed.Mode)
	}
	if len(parsed.SourceFiles) != 1 || parsed.SourceFiles[0] != "test.c" {
		t.Errorf("expected SourceFiles=[test.c], got %v", parsed.SourceFiles)
	}
	if parsed.OutputFile != "test.o" {
		t.Errorf("expected OutputFile=test.o, got %s", parsed.OutputFile)
	}
}

func TestParseCommandLineLinkOnly(t *testing.T) {
	args := []string{"test.o", "main.o", "-o", "app", "-lm"}
	parsed := ParseCommandLine(args)

	if parsed.Mode != ModeCompileAndLink {
		t.Errorf("expected ModeCompileAndLink, got %v", parsed.Mode)
	}
	if len(parsed.ObjectFiles) != 2 {
		t.Errorf("expected 2 object files, got %d", len(parsed.ObjectFiles))
	}
	if len(parsed.Libraries) != 1 || parsed.Libraries[0] != "m" {
		t.Errorf("expected Libraries=[m], got %v", parsed.Libraries)
	}
}

func TestParseCommandLineIncludeAndDefine(t *testing.T) {
	args := []string{"test.c", "-I/usr/include", "-D", "DEBUG", "-DVERSION=1"}
	parsed := ParseCommandLine(args)

	if len(parsed.IncludeDirs) != 1 || parsed.IncludeDirs[0] != "/usr/include" {
		t.Errorf("expected IncludeDirs=[/usr/include], got %v", parsed.IncludeDirs)
	}
	if len(parsed.Defines) != 2 {
		t.Errorf("expected 2 defines, got %d", len(parsed.Defines))
	}
}

func TestParseCommandLineCompilerFlags(t *testing.T) {
	args := []string{"test.c", "-O2", "-Wall", "-fPIC", "-std=c99"}
	parsed := ParseCommandLine(args)

	if len(parsed.CompilerFlags) != 4 {
		t.Errorf("expected 4 compiler flags, got %d", len(parsed.CompilerFlags))
	}
}

func TestParseCommandLineLinkerFlags(t *testing.T) {
	args := []string{"test.o", "-Wl,--as-needed", "-L/lib", "-lc"}
	parsed := ParseCommandLine(args)

	if len(parsed.LinkerFlags) < 1 {
		t.Errorf("expected linker flags, got none")
	}
	if len(parsed.LibraryDirs) != 1 || parsed.LibraryDirs[0] != "/lib" {
		t.Errorf("expected LibraryDirs=[/lib], got %v", parsed.LibraryDirs)
	}
}

func TestParseCommandLineSharedLibrary(t *testing.T) {
	args := []string{"test.o", "-shared", "-o", "libtest.so"}
	parsed := ParseCommandLine(args)

	if !parsed.IsSharedLibrary {
		t.Errorf("expected IsSharedLibrary=true, got false")
	}
}

func TestParseCommandLineMultipleSources(t *testing.T) {
	args := []string{"file1.c", "file2.c", "file3.c", "-o", "app"}
	parsed := ParseCommandLine(args)

	if len(parsed.SourceFiles) != 3 {
		t.Errorf("expected 3 source files, got %d", len(parsed.SourceFiles))
	}
	if parsed.OutputFile != "app" {
		t.Errorf("expected OutputFile=app, got %s", parsed.OutputFile)
	}
}

func TestParseCommandLineMixedArgs(t *testing.T) {
	args := []string{
		"main.c", "util.c",
		"-o", "myapp",
		"-I/usr/include/foo",
		"-DDEBUG=1",
		"-O3",
		"-Wall",
		"-lm", "-lpthread",
		"-L/usr/lib64",
	}
	parsed := ParseCommandLine(args)

	if len(parsed.SourceFiles) != 2 {
		t.Errorf("expected 2 source files, got %d", len(parsed.SourceFiles))
	}
	if parsed.OutputFile != "myapp" {
		t.Errorf("expected OutputFile=myapp, got %s", parsed.OutputFile)
	}
	if len(parsed.IncludeDirs) != 1 {
		t.Errorf("expected 1 include dir, got %d", len(parsed.IncludeDirs))
	}
	if len(parsed.Defines) != 1 {
		t.Errorf("expected 1 define, got %d", len(parsed.Defines))
	}
	if len(parsed.Libraries) != 2 {
		t.Errorf("expected 2 libraries, got %d", len(parsed.Libraries))
	}
	if len(parsed.LibraryDirs) != 1 {
		t.Errorf("expected 1 library dir, got %d", len(parsed.LibraryDirs))
	}
}

func TestDetectLanguageC(t *testing.T) {
	sourceFiles := []string{"main.c", "util.c"}
	isCpp := DetectLanguage(sourceFiles)

	if isCpp {
		t.Errorf("expected C language, got C++")
	}
}

func TestDetectLanguageCpp(t *testing.T) {
	tests := [][]string{
		{"main.cpp"},
		{"main.cc"},
		{"main.cxx"},
		{"main.c++"},
		{"file1.c", "file2.cpp"}, // Mixed: should detect C++
	}

	for _, sourceFiles := range tests {
		isCpp := DetectLanguage(sourceFiles)
		if !isCpp {
			t.Errorf("expected C++ language for %v, got C", sourceFiles)
		}
	}
}

func TestIsCompilerOnlyFlag(t *testing.T) {
	tests := map[string]bool{
		"-O2":             true,
		"-Wall":           true,
		"-fPIC":           true,
		"-std=c99":        true,
		"-Werror":         true,
		"-g":              true,
		"-ggdb":           true,
		"-Wl,--as-needed": false,
		"-l":              false,
		"-L/lib":          false,
	}

	for flag, expectedCompiler := range tests {
		result := isCompilerOnlyFlag(flag)
		if result != expectedCompiler {
			t.Errorf("flag %q: expected compiler=%v, got %v", flag, expectedCompiler, result)
		}
	}
}

func TestStripExtension(t *testing.T) {
	tests := map[string]string{
		"test.c":   "test",
		"main.cpp": "main",
		"file.o":   "file",
		"a/b/c.c":  "a/b/c",
		"file":     "file",
	}

	for input, expected := range tests {
		result := stripExtension(input)
		if result != expected {
			t.Errorf("stripExtension(%q): expected %q, got %q", input, expected, result)
		}
	}
}

func TestGenerateJobID(t *testing.T) {
	id := generateJobID()
	if id == "" {
		t.Errorf("expected non-empty job ID, got empty string")
	}
	if len(id) < 5 {
		t.Errorf("expected job ID to start with gcc- or gxx-, got %s", id)
	}
}
