package gcc_common

import (
	"context"
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

// JobSubmissionContext holds C/C++ specific information needed to construct a job.
type JobSubmissionContext struct {
	Runtime         *v1.Runtime
	SourceFiles     []string
	ObjectFiles     []string
	CompilerFlags   []string
	IncludeDirs     []string
	Defines         []string
	Libraries       []string // Named libraries passed with -l flag (e.g., "m" for -lm)
	LibraryFiles    []string // Full-path library files (e.g., "lib/libmath.a")
	LibraryDirs     []string
	LinkerFlags     []string
	OutputFile      string
	IsLinkJob       bool
	IsSharedLibrary bool
	Timeout         time.Duration
	WorkDir         string
}

func (jsc *JobSubmissionContext) createCppCompileJob() *v1.CppCompileJob {
	return &v1.CppCompileJob{
		SourceFiles:  jsc.SourceFiles,
		CompilerArgs: jsc.CompilerFlags,
		IncludeDirs:  jsc.IncludeDirs,
		Defines:      jsc.Defines,
		OutputFile:   jsc.OutputFile,
	}
}

func (jsc *JobSubmissionContext) createCppLinkJob() *v1.CppLinkJob {
	return &v1.CppLinkJob{
		ObjectFiles:     jsc.ObjectFiles,
		Libraries:       jsc.Libraries,
		LibraryFiles:    jsc.LibraryFiles,
		LibraryDirs:     jsc.LibraryDirs,
		LinkerArgs:      jsc.LinkerFlags,
		OutputFile:      jsc.OutputFile,
		IsSharedLibrary: jsc.IsSharedLibrary,
	}
}

func (jsc *JobSubmissionContext) createJob(ctx context.Context) (*v1.Job, error) {
	jobID := uuid.New().String()

	inputDataIDs := make([]string, 0, len(jsc.SourceFiles)+len(jsc.ObjectFiles)+len(jsc.LibraryFiles))
	inputDataIDs = append(inputDataIDs, jsc.SourceFiles...)
	if jsc.IsLinkJob {
		// For link jobs, include object files and library files as inputs
		inputDataIDs = append(inputDataIDs, jsc.ObjectFiles...)
		inputDataIDs = append(inputDataIDs, jsc.LibraryFiles...)
	}

	// Create JobData for all input files
	stager := drivers.NewJobDataStager(jsc.WorkDir)
	jobDataInputs, err := stager.CreateJobDataListForFiles(ctx, inputDataIDs, drivers.JobDataModeReference)
	if err != nil {
		return nil, err
	}

	logger := logging.Log().Child("drivers").Child("gcc_common")
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
		Id:   jsc.OutputFile,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path: jsc.OutputFile,
				Mode: 0644, // Default file permissions (will be set by compiler)
				// Content: nil (empty - daemon or runtime will write)
				// ContentHash: empty (will be computed after execution)
			},
		},
		CreatedAt: &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
	}
	jobDataOutputs := []*v1.JobData{outputJobData}
	logger.Debug("Created JobData output", "path", jsc.OutputFile)

	timeoutProto := &v1.TimeDuration{
		Count: int64(jsc.Timeout.Seconds()),
		Unit:  v1.TimeUnit_TIME_UNIT_SECOND,
	}

	var job *v1.Job
	if jsc.IsLinkJob {
		job = &v1.Job{
			Id:                 jobID,
			RuntimeRequirement: &v1.Job_Runtime{Runtime: jsc.Runtime},
			InputDataIds:       nil, // Don't populate - use Inputs instead (which have relative paths)
			JobSpec:            &v1.Job_CppLink{CppLink: jsc.createCppLinkJob()},
			SourceClientId:     "",
			SubmittedAt:        &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
			Timeout:            timeoutProto,
			Inputs:             jobDataInputs,
			Outputs:            jobDataOutputs,
			Cwd:                jsc.WorkDir,
		}
	} else {
		job = &v1.Job{
			Id:                 jobID,
			RuntimeRequirement: &v1.Job_Runtime{Runtime: jsc.Runtime},
			InputDataIds:       nil, // Don't populate - use Inputs instead (which have relative paths)
			JobSpec:            &v1.Job_CppCompile{CppCompile: jsc.createCppCompileJob()},
			SourceClientId:     "",
			SubmittedAt:        &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
			Timeout:            timeoutProto,
			Inputs:             jobDataInputs,
			Outputs:            jobDataOutputs,
			Cwd:                jsc.WorkDir,
		}
	}
	return job, nil
}

// CreateCppJob builds a Job proto from parsed C/C++ command-line arguments,
// the resolved runtime, and the working directory.
func CreateCppJob(ctx context.Context, parsed *ParsedArgs, runtime *v1.Runtime, workDir string) (*v1.Job, error) {
	logger := logging.Log().Child("drivers").Child("gcc_common")

	// Log parsed arguments breakdown
	logger.Debug("Parsed C/C++ command line arguments",
		"mode", modeString(parsed.Mode),
		"sourceFileCount", len(parsed.SourceFiles),
		"objectFileCount", len(parsed.ObjectFiles),
		"defineCount", len(parsed.Defines),
		"includeCount", len(parsed.IncludeDirs),
		"libraryCount", len(parsed.Libraries),
		"libDirCount", len(parsed.LibraryDirs),
		"compilerFlagCount", len(parsed.CompilerFlags),
		"linkerFlagCount", len(parsed.LinkerFlags),
		"isSharedLibrary", parsed.IsSharedLibrary,
		"outputFile", parsed.OutputFile,
	)

	// Log details of each category
	if len(parsed.SourceFiles) > 0 {
		logger.Debug("Source files", "files", parsed.SourceFiles)
	}
	if len(parsed.ObjectFiles) > 0 {
		logger.Debug("Object files (inputs)", "files", parsed.ObjectFiles)
	}
	if len(parsed.IncludeDirs) > 0 {
		logger.Debug("Include directories", "dirs", parsed.IncludeDirs)
	}
	if len(parsed.Defines) > 0 {
		logger.Debug("Defines/macros", "defines", parsed.Defines)
	}
	if len(parsed.Libraries) > 0 {
		logger.Debug("Libraries to link", "libs", parsed.Libraries)
	}
	if len(parsed.LibraryDirs) > 0 {
		logger.Debug("Library search directories", "dirs", parsed.LibraryDirs)
	}
	if len(parsed.CompilerFlags) > 0 {
		logger.Debug("Compiler flags", "flags", parsed.CompilerFlags)
	}
	if len(parsed.LinkerFlags) > 0 {
		logger.Debug("Linker flags", "flags", parsed.LinkerFlags)
	}

	// Log resolved runtime
	cppSpec, ok := runtime.ToolchainSpec.(*v1.Runtime_Cpp)
	cppCompiler := "unknown"
	cppVersion := "unknown"
	if ok && cppSpec.Cpp != nil {
		cppCompiler = cppSpec.Cpp.Compiler.String()
		if cppSpec.Cpp.CompilerVersion != nil {
			cppVersion = cppSpec.Cpp.CompilerVersion.String()
		}
	}
	logger.Debug("Using resolved runtime",
		"runtimeID", runtime.Id,
		"platform", runtime.Platform,
		"toolchain", runtime.Toolchain,
		"compiler", cppCompiler,
		"compilerVersion", cppVersion,
	)

	jsc := &JobSubmissionContext{
		Runtime:         runtime,
		SourceFiles:     normalizePaths(parsed.SourceFiles, workDir),
		ObjectFiles:     normalizePaths(parsed.ObjectFiles, workDir),
		CompilerFlags:   parsed.CompilerFlags,
		IncludeDirs:     normalizePaths(parsed.IncludeDirs, workDir),
		Defines:         parsed.Defines,
		Libraries:       parsed.Libraries,
		LibraryFiles:    normalizePaths(parsed.LibraryFiles, workDir),
		LibraryDirs:     normalizePaths(parsed.LibraryDirs, workDir),
		LinkerFlags:     parsed.LinkerFlags,
		OutputFile:      normalizePathToRelative(parsed.OutputFile, workDir),
		IsLinkJob:       parsed.Mode == ModeLink,
		IsSharedLibrary: parsed.IsSharedLibrary,
		Timeout:         5 * time.Minute,
		WorkDir:         workDir,
	}

	job, err := jsc.createJob(ctx)

	if err == nil && job != nil {
		// Log the created job structure
		logger.Debug("Created job for submission",
			"jobID", job.Id,
			"workDir", job.Cwd,
			"inputCount", len(job.Inputs),
			"outputCount", len(job.Outputs),
			"timeout", job.Timeout,
		)

		// Log input and output details
		for i, input := range job.Inputs {
			if fileData, ok := input.Data.(*v1.JobData_File); ok {
				logger.Debug("Job input", "index", i, "path", fileData.File.Path, "mode", fileData.File.Mode)
			}
		}
		for i, output := range job.Outputs {
			if fileData, ok := output.Data.(*v1.JobData_File); ok {
				logger.Debug("Job output", "index", i, "path", fileData.File.Path, "mode", fileData.File.Mode)
			}
		}

		// Log the job spec details
		if linkJob, ok := job.JobSpec.(*v1.Job_CppLink); ok {
			if linkJob.CppLink != nil {
				logger.Debug("CppLink job spec",
					"objectFileCount", len(linkJob.CppLink.ObjectFiles),
					"libraryCount", len(linkJob.CppLink.Libraries),
					"libraryFileCount", len(linkJob.CppLink.LibraryFiles),
					"libDirCount", len(linkJob.CppLink.LibraryDirs),
					"linkerArgCount", len(linkJob.CppLink.LinkerArgs),
					"outputFile", linkJob.CppLink.OutputFile,
					"isSharedLibrary", linkJob.CppLink.IsSharedLibrary,
				)
				if len(linkJob.CppLink.ObjectFiles) > 0 {
					logger.Debug("CppLink object files (job inputs)", "files", linkJob.CppLink.ObjectFiles)
				}
				if len(linkJob.CppLink.LibraryFiles) > 0 {
					logger.Debug("CppLink library files (job inputs)", "files", linkJob.CppLink.LibraryFiles)
				}
				if len(linkJob.CppLink.Libraries) > 0 {
					logger.Debug("CppLink libraries (job tool inputs)", "libs", linkJob.CppLink.Libraries)
				}
				if len(linkJob.CppLink.LibraryDirs) > 0 {
					logger.Debug("CppLink library search dirs (job tool arguments)", "dirs", linkJob.CppLink.LibraryDirs)
				}
				if len(linkJob.CppLink.LinkerArgs) > 0 {
					logger.Debug("CppLink tool arguments (linker flags from CLI)", "args", linkJob.CppLink.LinkerArgs)
				}
			}
		} else if compileJob, ok := job.JobSpec.(*v1.Job_CppCompile); ok {
			if compileJob.CppCompile != nil {
				logger.Debug("CppCompile job spec",
					"sourceFileCount", len(compileJob.CppCompile.SourceFiles),
					"includeCount", len(compileJob.CppCompile.IncludeDirs),
					"defineCount", len(compileJob.CppCompile.Defines),
					"compilerArgCount", len(compileJob.CppCompile.CompilerArgs),
					"outputFile", compileJob.CppCompile.OutputFile,
				)
				if len(compileJob.CppCompile.SourceFiles) > 0 {
					logger.Debug("CppCompile source files (job inputs)", "files", compileJob.CppCompile.SourceFiles)
				}
				if len(compileJob.CppCompile.IncludeDirs) > 0 {
					logger.Debug("CppCompile include directories (job tool arguments)", "dirs", compileJob.CppCompile.IncludeDirs)
				}
				if len(compileJob.CppCompile.Defines) > 0 {
					logger.Debug("CppCompile defines/macros (job tool arguments)", "defines", compileJob.CppCompile.Defines)
				}
				if len(compileJob.CppCompile.CompilerArgs) > 0 {
					logger.Debug("CppCompile tool arguments (compiler flags from CLI)", "args", compileJob.CppCompile.CompilerArgs)
				}
			}
		}
	}

	return job, err
}

// modeString converts a CompileMode to a human-readable string
func modeString(mode CompileMode) string {
	switch mode {
	case ModeCompileOnly:
		return "compile-only"
	case ModeLink:
		return "link-only"
	case ModeCompileAndLink:
		return "compile-and-link"
	default:
		return "unknown"
	}
}
