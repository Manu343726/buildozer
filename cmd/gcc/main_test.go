package main

import (
	"context"
	"testing"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

func TestCreateJobCompile(t *testing.T) {
	args := []string{"test.c", "-c", "-o", "test.o", "-I/usr/include", "-DDEBUG"}
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

	if cppToolchain.Cpp.Language != v1.CppLanguage_CPP_LANGUAGE_C {
		t.Errorf("expected C language, got %v", cppToolchain.Cpp.Language)
	}

	if cppToolchain.Cpp.Compiler != v1.CppCompiler_CPP_COMPILER_GCC {
		t.Errorf("expected GCC compiler, got %v", cppToolchain.Cpp.Compiler)
	}

	compileJob, ok := job.JobSpec.(*v1.Job_CppCompile)
	if !ok {
		t.Fatalf("expected CppCompileJob in job spec, got %T", job.JobSpec)
	}

	if len(compileJob.CppCompile.SourceFiles) != 1 || compileJob.CppCompile.SourceFiles[0] != "test.c" {
		t.Errorf("expected SourceFiles=[test.c], got %v", compileJob.CppCompile.SourceFiles)
	}

	if compileJob.CppCompile.OutputFile != "test.o" {
		t.Errorf("expected OutputFile=test.o, got %s", compileJob.CppCompile.OutputFile)
	}
}

func TestCreateJobLink(t *testing.T) {
	args := []string{"test.o", "main.o", "-o", "app", "-lm"}
	parsed := ParseCommandLine(args)

	job, err := createJob(parsed)
	if err != nil {
		t.Fatalf("createJob failed: %v", err)
	}

	linkJob, ok := job.JobSpec.(*v1.Job_CppLink)
	if !ok {
		t.Fatalf("expected CppLinkJob in job spec, got %T", job.JobSpec)
	}

	if len(linkJob.CppLink.ObjectFiles) != 2 {
		t.Errorf("expected 2 object files, got %d", len(linkJob.CppLink.ObjectFiles))
	}

	if linkJob.CppLink.OutputFile != "app" {
		t.Errorf("expected OutputFile=app, got %s", linkJob.CppLink.OutputFile)
	}

	if len(linkJob.CppLink.Libraries) != 1 || linkJob.CppLink.Libraries[0] != "m" {
		t.Errorf("expected Libraries=[m], got %v", linkJob.CppLink.Libraries)
	}
}

func TestCreateJobAutoOutputFile(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{"compile only", []string{"test.c", "-c"}, "test.o"},
		{"link only", []string{"test.o", "main.o"}, "a.out"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed := ParseCommandLine(test.args)
			job, err := createJob(parsed)
			if err != nil {
				t.Fatalf("createJob failed: %v", err)
			}

			var outputFile string
			switch spec := job.JobSpec.(type) {
			case *v1.Job_CppCompile:
				outputFile = spec.CppCompile.OutputFile
			case *v1.Job_CppLink:
				outputFile = spec.CppLink.OutputFile
			}

			if outputFile != test.expected {
				t.Errorf("expected OutputFile=%s, got %s", test.expected, outputFile)
			}
		})
	}
}

func TestCreateJobTimeout(t *testing.T) {
	args := []string{"test.c", "-c"}
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

func TestExecuteJobReturnsResult(t *testing.T) {
	args := []string{"test.c", "-c"}
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
