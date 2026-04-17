package ar_common

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/drivers"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/google/uuid"
)

// normalizePathToRelative converts an absolute path to relative, otherwise returns as-is.
// If the path is absolute and can be made relative to workDir, returns the relative path.
// Otherwise returns the original path.
func normalizePathToRelative(filePath, workDir string) string {
	if !filepath.IsAbs(filePath) {
		// Already relative
		return filePath
	}

	// Try to make it relative to workDir
	if workDir != "" {
		if relPath, err := filepath.Rel(workDir, filePath); err == nil {
			return relPath
		}
	}

	// If conversion fails, return original
	return filePath
}

// normalizePaths converts a list of paths to relative paths.
func normalizePaths(paths []string, workDir string) []string {
	result := make([]string, len(paths))
	for i, path := range paths {
		result[i] = normalizePathToRelative(path, workDir)
	}
	return result
}

// CreateArchiveJob creates a Job protocol buffer from parsed ar arguments.
func CreateArchiveJob(ctx context.Context, parsed *ParsedArgs, runtime *v1.Runtime, workDir string) (*v1.Job, error) {
	if parsed == nil {
		return nil, fmt.Errorf("parsed args is nil")
	}
	if runtime == nil {
		return nil, fmt.Errorf("runtime is nil")
	}

	logger := logging.Log().Child("drivers").Child("ar")

	// Generate unique job ID
	jobID := uuid.New().String()

	archiveJob := &v1.CppArchiveJob{
		InputFiles: normalizePaths(parsed.InputFiles, workDir),
		OutputFile: normalizePathToRelative(parsed.OutputFile, workDir),
		ArFlags:    parsed.Flags,
	}

	job := &v1.Job{
		Id: jobID,
		RuntimeRequirement: &v1.Job_Runtime{
			Runtime: runtime,
		},
		Cwd: workDir,
		JobSpec: &v1.Job_CppArchive{
			CppArchive: archiveJob,
		},
		SubmittedAt: &v1.TimeStamp{UnixMillis: 0}, // Will be set by daemon
	}

	logger.Info("created archive job",
		"jobID", jobID,
		"outputFile", parsed.OutputFile,
		"inputCount", len(parsed.InputFiles),
		"flags", fmt.Sprintf("%v", parsed.Flags),
	)

	return job, nil
}

// CreateArchiveJobWithRuntimeABI extracts ABI requirements from a resolved runtime
// and creates a RuntimeMatchQuery Job targeting any runtime with matching ABI.
func CreateArchiveJobWithRuntimeABI(ctx context.Context, parsed *ParsedArgs, runtime *v1.Runtime, workDir string) (*v1.Job, error) {
	if parsed == nil {
		return nil, fmt.Errorf("parsed args is nil")
	}
	if runtime == nil {
		return nil, fmt.Errorf("runtime is nil")
	}

	logger := logging.Log().Child("drivers").Child("ar")

	// Generate unique job ID
	jobID := uuid.New().String()

	// Extract ABI requirements from the Runtime's CppToolchain
	var cruntime, cruntimeVersion, arch string

	if cppSpec, ok := runtime.ToolchainSpec.(*v1.Runtime_Cpp); ok && cppSpec.Cpp != nil {
		cpp := cppSpec.Cpp

		// Extract c_runtime enum name
		cruntimeStr := cpp.CRuntime.String()
		if cruntimeStr == "C_RUNTIME_UNSPECIFIED" {
			return nil, fmt.Errorf("runtime c_runtime is unspecified")
		}
		cruntime = cruntimeStr

		// Extract c_runtime_version
		if cpp.CRuntimeVersion == nil {
			return nil, fmt.Errorf("runtime c_runtime_version is nil")
		}
		cruntimeVersion = cpp.CRuntimeVersion.String()

		// Extract architecture enum name
		archStr := cpp.Architecture.String()
		if archStr == "CPU_ARCHITECTURE_UNSPECIFIED" {
			return nil, fmt.Errorf("runtime architecture is unspecified")
		}
		arch = archStr
	} else {
		return nil, fmt.Errorf("runtime toolchain spec is not CppToolchain or is nil")
	}

	// Construct RuntimeMatchQuery: AR accepts both C and C++ runtimes with matching ABI
	runtimeMatchQuery := &v1.RuntimeMatchQuery{
		// AR works with both C and C++ toolchains
		Toolchains: []v1.RuntimeToolchain{
			v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_C,
			v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
		},
		// AR is ABI-agnostic - only cares about c_runtime and architecture
		Params: map[string]*v1.StringArray{
			"c_runtime": {
				Values: []string{cruntime},
			},
			"c_runtime_version": {
				Values: []string{cruntimeVersion},
			},
			"architecture": {
				Values: []string{arch},
			},
			// Empty values for optional params = don't care about their values
			"cpp_stdlib": {
				Values: []string{}, // Accept any C++ stdlib or none
			},
		},
	}

	archiveJob := &v1.CppArchiveJob{
		InputFiles: normalizePaths(parsed.InputFiles, workDir),
		OutputFile: normalizePathToRelative(parsed.OutputFile, workDir),
		ArFlags:    parsed.Flags,
	}

	job := &v1.Job{
		Id: jobID,
		RuntimeRequirement: &v1.Job_RuntimeMatchQuery{
			RuntimeMatchQuery: runtimeMatchQuery,
		},
		Cwd: workDir,
		JobSpec: &v1.Job_CppArchive{
			CppArchive: archiveJob,
		},
		SubmittedAt: &v1.TimeStamp{UnixMillis: 0}, // Will be set by daemon
	}

	logger.Info("created archive job with runtime match query from resolved runtime",
		"jobID", jobID,
		"outputFile", parsed.OutputFile,
		"inputCount", len(parsed.InputFiles),
		"flags", fmt.Sprintf("%v", parsed.Flags),
		"c_runtime", cruntime,
		"c_runtime_version", cruntimeVersion,
		"architecture", arch,
	)

	return job, nil
}

// CreateArchiveJobWithRuntimeABIFromConfig extracts ABI requirements from driver config
// and creates a RuntimeMatchQuery Job targeting any runtime with matching ABI.
// This avoids the need to resolve a runtime just to query for compatible runtimes.
func CreateArchiveJobWithRuntimeABIFromConfig(ctx context.Context, parsed *ParsedArgs, cfg *ArConfig, workDir string) (*v1.Job, error) {
	if parsed == nil {
		return nil, fmt.Errorf("parsed args is nil")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	logger := logging.Log().Child("drivers").Child("ar")

	// Generate unique job ID
	jobID := uuid.New().String()

	// Construct RuntimeMatchQuery: AR accepts both C and C++ runtimes with matching ABI
	runtimeMatchQuery := &v1.RuntimeMatchQuery{
		// AR works with both C and C++ toolchains
		Toolchains: []v1.RuntimeToolchain{
			v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_C,
			v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
		},
		// AR is ABI-agnostic - only cares about c_runtime and architecture
		Params: map[string]*v1.StringArray{
			"c_runtime": {
				Values: []string{string(cfg.CRuntime)},
			},
			"c_runtime_version": {
				Values: []string{cfg.CRuntimeVersion},
			},
			"architecture": {
				Values: []string{string(cfg.Architecture)},
			},
			// Empty values for optional params = don't care about their values
			"cpp_stdlib": {
				Values: []string{}, // Accept any C++ stdlib or none
			},
		},
	}

	// Create JobData for all input files
	stager := drivers.NewJobDataStager(workDir)
	jobDataInputs, err := stager.CreateJobDataListForFiles(ctx, parsed.InputFiles, drivers.JobDataModeReference)
	if err != nil {
		return nil, err
	}

	logger.Debug("Created JobData inputs", "count", len(jobDataInputs))
	for i, jd := range jobDataInputs {
		if jd.Data != nil {
			if fileData, ok := jd.Data.(*v1.JobData_File); ok {
				logger.Debug("Input file", "index", i, "path", fileData.File.Path)
			}
		}
	}

	// Create JobData for output file
	// Output files are created without content initially - daemon/runtime will fill them
	outputJobData := &v1.JobData{
		Id:   parsed.OutputFile,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path: parsed.OutputFile,
				Mode: 0644, // Default file permissions (will be set by compiler)
				// Content: nil (empty - daemon or runtime will write)
				// ContentHash: empty (will be computed after execution)
			},
		},
		CreatedAt: &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
	}
	jobDataOutputs := []*v1.JobData{outputJobData}
	logger.Debug("Created JobData output", "path", parsed.OutputFile)

	archiveJob := &v1.CppArchiveJob{
		InputFiles: normalizePaths(parsed.InputFiles, workDir),
		OutputFile: normalizePathToRelative(parsed.OutputFile, workDir),
		ArFlags:    parsed.Flags,
	}

	job := &v1.Job{
		Id: jobID,
		RuntimeRequirement: &v1.Job_RuntimeMatchQuery{
			RuntimeMatchQuery: runtimeMatchQuery,
		},
		Cwd: workDir,
		JobSpec: &v1.Job_CppArchive{
			CppArchive: archiveJob,
		},
		Inputs:      jobDataInputs,
		Outputs:     jobDataOutputs,
		SubmittedAt: &v1.TimeStamp{UnixMillis: 0}, // Will be set by daemon
	}

	logger.Info("created archive job with runtime match query from config",
		"jobID", jobID,
		"outputFile", parsed.OutputFile,
		"inputCount", len(parsed.InputFiles),
		"flags", fmt.Sprintf("%v", parsed.Flags),
		"c_runtime", cfg.CRuntime,
		"c_runtime_version", cfg.CRuntimeVersion,
		"architecture", cfg.Architecture,
	)

	// Log the archive job structure
	logger.Debug("Created CppArchive job spec",
		"inputFileCount", len(archiveJob.InputFiles),
		"outputFile", archiveJob.OutputFile,
		"arFlagCount", len(archiveJob.ArFlags),
	)

	if len(archiveJob.InputFiles) > 0 {
		logger.Debug("CppArchive input files (job inputs)", "files", archiveJob.InputFiles)
	}
	if len(archiveJob.ArFlags) > 0 {
		logger.Debug("CppArchive tool arguments (ar flags from CLI)", "flags", archiveJob.ArFlags)
	}

	logger.Debug("Archive runtime match query",
		"toolchainCount", len(runtimeMatchQuery.Toolchains),
		"paramConstraintCount", len(runtimeMatchQuery.Params),
	)

	return job, nil
}
