package gcc_common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCommandLine_CompileOnly(t *testing.T) {
	// gcc -c -o obj/main.o src/main.c
	args := []string{"-c", "-o", "obj/main.o", "src/main.c"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeCompileOnly, parsed.Mode, "should detect compile-only mode")
	assert.ElementsMatch(t, []string{"src/main.c"}, parsed.SourceFiles, "should extract source file")
	assert.Empty(t, parsed.ObjectFiles, "should have no object files")
	assert.Equal(t, "obj/main.o", parsed.OutputFile, "should extract output file")
}

func TestParseCommandLine_LinkOnly(t *testing.T) {
	// g++ obj/main.o -o bin/myapp -lm
	args := []string{"obj/main.o", "-o", "bin/myapp", "-lm"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeLink, parsed.Mode, "should detect link-only mode when no source files")
	assert.Empty(t, parsed.SourceFiles, "should have no source files")
	assert.ElementsMatch(t, []string{"obj/main.o"}, parsed.ObjectFiles, "should extract object file")
	assert.ElementsMatch(t, []string{"m"}, parsed.Libraries, "should extract library")
	assert.Equal(t, "bin/myapp", parsed.OutputFile, "should extract output file")
}

func TestParseCommandLine_LinkMultipleObjectFiles(t *testing.T) {
	// g++ obj/main.o obj/util.o -o bin/myapp
	args := []string{"obj/main.o", "obj/util.o", "-o", "bin/myapp"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeLink, parsed.Mode, "should detect link-only mode")
	assert.Len(t, parsed.ObjectFiles, 2, "should have 2 object files")
	assert.ElementsMatch(t, []string{"obj/main.o", "obj/util.o"}, parsed.ObjectFiles, "should extract all object files")
}

func TestParseCommandLine_CompileAndLink(t *testing.T) {
	// gcc -o bin/myapp src/main.c src/util.c
	args := []string{"-o", "bin/myapp", "src/main.c", "src/util.c"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeCompileAndLink, parsed.Mode, "should detect compile-and-link mode")
	assert.Len(t, parsed.SourceFiles, 2, "should have 2 source files")
	assert.ElementsMatch(t, []string{"src/main.c", "src/util.c"}, parsed.SourceFiles, "should extract both source files")
	assert.Equal(t, "bin/myapp", parsed.OutputFile, "should extract output file")
}

func TestParseCommandLine_MixedSourceAndObject(t *testing.T) {
	// gcc src/main.c obj/util.o -o bin/myapp
	// This is compile+link: compile main.c and link with util.o
	args := []string{"src/main.c", "obj/util.o", "-o", "bin/myapp"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeCompileAndLink, parsed.Mode, "should detect compile-and-link mode when mix of source and object files")
	assert.ElementsMatch(t, []string{"src/main.c"}, parsed.SourceFiles, "should extract source file")
	assert.ElementsMatch(t, []string{"obj/util.o"}, parsed.ObjectFiles, "should extract object file")
}

func TestParseCommandLine_WithLibraries(t *testing.T) {
	// gcc -o bin/myapp src/main.c -lm -lpthread
	args := []string{"-o", "bin/myapp", "src/main.c", "-lm", "-lpthread"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.ElementsMatch(t, []string{"src/main.c"}, parsed.SourceFiles, "should extract source file")
	assert.Len(t, parsed.Libraries, 2, "should have 2 libraries")
	assert.ElementsMatch(t, []string{"m", "pthread"}, parsed.Libraries, "should extract libraries without -l prefix")
}

func TestParseCommandLine_WithLibraryDirs(t *testing.T) {
	// gcc -o bin/myapp src/main.c -L/usr/local/lib -L./lib -lm
	args := []string{"-o", "bin/myapp", "src/main.c", "-L/usr/local/lib", "-L./lib", "-lm"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Len(t, parsed.LibraryDirs, 2, "should have 2 library directories")
	assert.ElementsMatch(t, []string{"/usr/local/lib", "./lib"}, parsed.LibraryDirs, "should extract library directories")
}

func TestParseCommandLine_WithIncludeAndDefines(t *testing.T) {
	// gcc -c -I/usr/include -I./include -DDEBUG -DVERSION=1 -o obj/main.o src/main.c
	args := []string{"-c", "-I/usr/include", "-I./include", "-DDEBUG", "-DVERSION=1", "-o", "obj/main.o", "src/main.c"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Len(t, parsed.IncludeDirs, 2, "should have 2 include directories")
	assert.ElementsMatch(t, []string{"/usr/include", "./include"}, parsed.IncludeDirs, "should extract include directories")
	assert.Len(t, parsed.Defines, 2, "should have 2 defines")
	assert.ElementsMatch(t, []string{"DEBUG", "VERSION=1"}, parsed.Defines, "should extract defines without -D prefix")
}

func TestParseCommandLine_WithCompilerFlags(t *testing.T) {
	// gcc -c -O2 -Wall -std=c99 -march=native -o obj/main.o src/main.c
	args := []string{"-c", "-O2", "-Wall", "-std=c99", "-march=native", "-o", "obj/main.o", "src/main.c"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Len(t, parsed.CompilerFlags, 4, "should have 4 compiler flags")
	assert.ElementsMatch(t, []string{"-O2", "-Wall", "-std=c99", "-march=native"}, parsed.CompilerFlags, "should extract compiler flags")
}

func TestParseCommandLine_WithLinkerFlags(t *testing.T) {
	// gcc src/main.c -static -o bin/myapp
	args := []string{"src/main.c", "-static", "-o", "bin/myapp"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Len(t, parsed.LinkerFlags, 1, "should have 1 linker flag")
	assert.ElementsMatch(t, []string{"-static"}, parsed.LinkerFlags, "should extract linker flags")
}

func TestParseCommandLine_SharedLibrary(t *testing.T) {
	// gcc -shared -fPIC src/main.c -o lib/libmylib.so
	args := []string{"-shared", "-fPIC", "src/main.c", "-o", "lib/libmylib.so"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.True(t, parsed.IsSharedLibrary, "should detect shared library flag")
	assert.Contains(t, parsed.LinkerFlags, "-shared", "should include -shared in linker flags")
}

func TestParseCommandLine_ArchiveFile(t *testing.T) {
	// Link with .a archive file
	args := []string{"obj/main.o", "lib/libmath_cpp.a", "lib/libmath_c.a", "-o", "bin/calc_cpp"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeLink, parsed.Mode, "should detect link mode")
	assert.ElementsMatch(t, []string{"obj/main.o"}, parsed.ObjectFiles, "object file should be in ObjectFiles")
	// Note: .a files are not currently categorized as libraries in the parser
	// (they're just left ungrouped), but they would be passed as input files
}

func TestParseCommandLine_CppExtensions(t *testing.T) {
	testCases := []struct {
		name         string
		filename     string
		isCppExt     bool
		isSourceFile bool
	}{
		{"cpp extension", "src/main.cpp", true, true},
		{"cc extension", "src/main.cc", true, true},
		{"cxx extension", "src/main.cxx", true, true},
		{"c++ extension", "src/main.c++", true, true},
		{"c extension", "src/main.c", false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"-c", "-o", "out.o", tc.filename}
			parsed := ParseCommandLine(args)

			if tc.isSourceFile {
				assert.ElementsMatch(t, []string{tc.filename}, parsed.SourceFiles, "should be recognized as source file")
				if tc.isCppExt {
					assert.True(t, DetectLanguage(parsed.SourceFiles), "C++ file should be detected as C++")
				} else {
					assert.False(t, DetectLanguage(parsed.SourceFiles), "C file should be detected as C")
				}
			}
		})
	}
}

func TestParseCommandLine_LibraryPathVariants(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		expected []string
	}{
		{"combined -L", []string{"-L/usr/lib", "src/main.c", "-o", "bin/app"}, []string{"/usr/lib"}},
		{"separate -L", []string{"-L", "/usr/lib", "src/main.c", "-o", "bin/app"}, []string{"/usr/lib"}},
		{"combined -l", []string{"-lm", "src/main.c", "-o", "bin/app"}, []string{"m"}},
		{"separate -l", []string{"-l", "m", "src/main.c", "-o", "bin/app"}, []string{"m"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed := ParseCommandLine(tc.args)
			require.NotNil(t, parsed, "parsed result should not be nil")
			if tc.name == "combined -L" || tc.name == "separate -L" {
				assert.ElementsMatch(t, tc.expected, parsed.LibraryDirs, "library directories should match")
			} else {
				assert.ElementsMatch(t, tc.expected, parsed.Libraries, "libraries should match")
			}
		})
	}
}

func TestDetectLanguage(t *testing.T) {
	testCases := []struct {
		name     string
		files    []string
		expected bool // true for C++, false for C
	}{
		{"C only", []string{"main.c"}, false},
		{"C++ only", []string{"main.cpp"}, true},
		{"C++ with .cc", []string{"main.cc"}, true},
		{"C++ with .cxx", []string{"main.cxx"}, true},
		{"C++ with .c++", []string{"main.c++"}, true},
		{"Multiple C files", []string{"main.c", "util.c"}, false},
		{"Multiple C++ files", []string{"main.cpp", "util.cpp"}, true},
		{"Mixed: C++ first", []string{"main.cpp", "util.c"}, true},
		{"Mixed: C first", []string{"main.c", "util.cpp"}, true},
		{"Empty", []string{}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := DetectLanguage(tc.files)
			assert.Equal(t, tc.expected, result, "language detection should match")
		})
	}
}

func TestIsCompilerOnlyFlag(t *testing.T) {
	testCases := []struct {
		flag           string
		isCompilerOnly bool
	}{
		{"-c", true},
		{"-S", true},
		{"-E", true},
		{"-M", true},
		{"-MM", true},
		{"-fPIC", true},
		{"-fPIE", true},
		{"-nostdinc", true},
		{"-std=c99", true},
		{"-march=native", true},
		{"-mtune=core2", true},
		{"-O2", true},
		{"-O3", true},
		{"-g", true},
		{"-Wall", true},
		{"-Werror", true},
		// Note: -Wl,* is classified as compiler flag because it starts with -W
		// but that's okay - it's still passed to the compiler which forwards it to linker
		{"-Wl,--as-needed", true},
		// Linker-only or generic flags
		{"-shared", false},
		{"-static", false},
		{"-pthread", false},
		{"-Lfoo", false},
		{"-lfoo", false},
	}

	for _, tc := range testCases {
		t.Run(tc.flag, func(t *testing.T) {
			result := isCompilerOnlyFlag(tc.flag)
			assert.Equal(t, tc.isCompilerOnly, result, "flag classification should match")
		})
	}
}

func TestParseCommandLine_RealWorldLinkJob(t *testing.T) {
	// This is the real command from the user's issue:
	// /workspaces/buildozer/examples/cmake_project/../../bin/g++ CMakeFiles/calc_cpp.dir/src/main_cpp.cpp.o -o bin/calc_cpp  lib/libmath_cpp.a lib/libmath_c.a
	args := []string{"CMakeFiles/calc_cpp.dir/src/main_cpp.cpp.o", "-o", "bin/calc_cpp", "lib/libmath_cpp.a", "lib/libmath_c.a"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeLink, parsed.Mode, "should detect link-only mode")
	assert.Empty(t, parsed.SourceFiles, "should have no source files")
	assert.ElementsMatch(t, []string{"CMakeFiles/calc_cpp.dir/src/main_cpp.cpp.o"}, parsed.ObjectFiles, "should extract object file")
	assert.ElementsMatch(t, []string{"lib/libmath_cpp.a", "lib/libmath_c.a"}, parsed.LibraryFiles, "should extract library files")
	assert.Empty(t, parsed.Libraries, "should have no named libraries")
	assert.Equal(t, "bin/calc_cpp", parsed.OutputFile, "should extract output file")
}

func TestParseCommandLine_EmptyArgs(t *testing.T) {
	parsed := ParseCommandLine([]string{})
	require.NotNil(t, parsed, "parsed result should not be nil")
	assert.Equal(t, ModeCompileAndLink, parsed.Mode, "default mode should be compile-and-link")
	assert.Empty(t, parsed.SourceFiles, "should have no source files")
	assert.Empty(t, parsed.ObjectFiles, "should have no object files")
	assert.Empty(t, parsed.OutputFile, "should have no output file")
}

func TestParseCommandLine_OnlyCompilerFlags(t *testing.T) {
	args := []string{"-O2", "-Wall", "-std=c99"}
	parsed := ParseCommandLine(args)

	require.NotNil(t, parsed, "parsed result should not be nil")
	// No source files, no object files, no -c flag = stays as compile-and-link (default)
	// This is an edge case that shouldn't happen in practice
	assert.Equal(t, ModeCompileAndLink, parsed.Mode, "should default to compile-and-link")
	assert.Empty(t, parsed.SourceFiles, "should have no source files")
	assert.Empty(t, parsed.ObjectFiles, "should have no object files")
}
