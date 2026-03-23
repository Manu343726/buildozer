package main

import (
	"context"
	"testing"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

func TestCreateJobCppCompile(t *testing.T) {
	args := []string{"test.cpp", "-c", "-o", "test.o", "-I/usr/include", "-DDEBUG"}
	parsed := ParseCommandLine(args)

	job, err := createJob(parsed)
	if err != nil {
		t.Fatalf("createJob failed: %v", err)
	}

	if job == nil {
		t.Fatalf("expected non-nil job, got nil")
	}

	if job.Runtime == nil {
		t.Fatalf("expected non-nil runtime, got nil")
	}

	if job.Runtime.Toolchain == nil {
		t.Fatalf("expected non-nil toolchain, got nil")
	}

	cppToolchain := job.Runtime.Toolchain.(*v1.Runtime_Cpp)
	if cppToolchain == nil {
		t.Fatalf("expected CppToolchain in runtime, got something else")
	}

	if cppToolchain.Cpp.Language != v1.CppLanguage_CPP_LANGUAGE_CPP {
		t.Errorf("expected C++ language, got %v", cppToolchain.Cpp.Language)
	}

	if cppToolchain.Cpp.Compiler != v1.CppCompiler_CPP_COMPILER_GCC {
		t.Errorf("expected GCC compiler, got %v", cppToolchain.Cpp.Compiler)
	}

	compileJob, ok := job.JobSpec.(*v1.Job_CppCompile)
	if !ok {
		t.Fatalf("expected CppCompileJob in job spec, got %T", job.JobSpec)
	}

	if len(compileJob.CppCompile.SourceFiles) != 1 || compileJob.CppCompile.SourceFiles[0] != "test.cpp" {
		t.Errorf("expected SourceFiles=[test.cpp], got %v", compileJob.CppCompile.SourceFiles)
	}

	if compileJob.CppCompile.OutputFile != "test.o" {
		t.Errorf("expected OutputFile=test.o, got %s", compileJob.CppCompile.OutputFile)
	}
}

func TestCreateJobCppSharedLibrary(t *testing.T) {
	args := []string{"test.cpp", "util.cpp", "-shared", "-o", "libtest.so"}
	parsed := ParseCommandLine(args)

	job, err := createJob(parsed)
	if err != nil {
		t.Fatalf("createJob failed: %v", err)
	}

	// When we have sources with -shared flag, it creates a CppCompileJob
	// that will compile and link in one step
	compileJob, ok := job.JobSpec.(*v1.Job_CppCompile)
	if !ok {
		t.Fatalf("expected CppCompileJob in job spec, got %T", job.JobSpec)
	}

	if len(compileJob.CppCompile.SourceFiles) != 2 {
		t.Errorf("expected 2 source files, got %d", len(compileJob.CppCompile.SourceFiles))
	}

	if compileJob.CppCompile.OutputFile != "libtest.so" {
		t.Errorf("expected OutputFile=libtest.so, got %s", compileJob.CppCompile.OutputFile)
	}

	// Check that -shared flag is included in compiler args
	hasShared := false
	for _, arg := range compileJob.CppCompile.CompilerArgs {
		if arg == "-shared" {
			hasShared = true
			break
		}
	}
	if !hasShared {
		t.Errorf("expected -shared flag in compiler args, got %v", compileJob.CppCompile.CompilerArgs)
	}
}

func TestCreateJobCppCompileTimeout(t *testing.T) {
	args := []string{"test.cpp", "-c"}
	parsed := ParseCommandLine(args)

	job, err := createJob(parsed)
	if err != nil {
		t.Fatalf("createJob failed: %v", err)
	}

	if job.Timeout == nil {
		t.Fatalf("expected non-nil timeout, got nil")
	}

	if job.Timeout.Count != 300 {
		t.Errorf("expected timeout count=300, got %d", job.Timeout.Count)
	}

	if job.Timeout.Unit != v1.TimeUnit_TIME_UNIT_SECOND {
		t.Errorf("expected timeout unit=SECOND, got %v", job.Timeout.Unit)
	}
}

func TestExecuteCppJob(t *testing.T) {
	args := []string{"test.cpp", "-c"}
	parsed := ParseCommandLine(args)

	job, err := createJob(parsed)
	if err != nil {
		t.Fatalf("createJob failed: %v", err)
	}

	ctx := context.Background()
	result, err := executeJob(ctx, job)
	if err != nil {
		t.Errorf("executeJob failed: %v", err)
	}

	if result == nil {
		t.Errorf("expected non-nil result, got nil")
	}
}
