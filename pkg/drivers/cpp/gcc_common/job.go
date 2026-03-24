package gcc_common

import (
	"context"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/google/uuid"
)

// JobSubmissionContext holds C/C++ specific information needed to construct a job.
type JobSubmissionContext struct {
	Runtime         *v1.Runtime
	SourceFiles     []string
	ObjectFiles     []string
	CompilerFlags   []string
	IncludeDirs     []string
	Defines         []string
	Libraries       []string
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
		LibraryDirs:     jsc.LibraryDirs,
		LinkerArgs:      jsc.LinkerFlags,
		OutputFile:      jsc.OutputFile,
		IsSharedLibrary: jsc.IsSharedLibrary,
	}
}

func (jsc *JobSubmissionContext) createJob(ctx context.Context) (*v1.Job, error) {
	jobID := uuid.New().String()

	inputDataIDs := make([]string, 0, len(jsc.SourceFiles)+len(jsc.ObjectFiles))
	inputDataIDs = append(inputDataIDs, jsc.SourceFiles...)
	if jsc.IsLinkJob {
		inputDataIDs = append(inputDataIDs, jsc.ObjectFiles...)
	}

	timeoutProto := &v1.TimeDuration{
		Count: int64(jsc.Timeout.Seconds()),
		Unit:  v1.TimeUnit_TIME_UNIT_SECOND,
	}

	var job *v1.Job
	if jsc.IsLinkJob {
		job = &v1.Job{
			Id:                    jobID,
			Runtime:               jsc.Runtime,
			InputDataIds:          inputDataIDs,
			ExpectedOutputDataIds: []string{jsc.OutputFile},
			JobSpec:               &v1.Job_CppLink{CppLink: jsc.createCppLinkJob()},
			SourceClientId:        "",
			SubmittedAt:           &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
			Timeout:               timeoutProto,
		}
	} else {
		job = &v1.Job{
			Id:                    jobID,
			Runtime:               jsc.Runtime,
			InputDataIds:          inputDataIDs,
			ExpectedOutputDataIds: []string{jsc.OutputFile},
			JobSpec:               &v1.Job_CppCompile{CppCompile: jsc.createCppCompileJob()},
			SourceClientId:        "",
			SubmittedAt:           &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
			Timeout:               timeoutProto,
		}
	}
	return job, nil
}

// CreateCppJob builds a Job proto from parsed C/C++ command-line arguments,
// the resolved runtime, and the working directory.
func CreateCppJob(ctx context.Context, parsed *ParsedArgs, runtime *v1.Runtime, workDir string) (*v1.Job, error) {
	jsc := &JobSubmissionContext{
		Runtime:         runtime,
		SourceFiles:     parsed.SourceFiles,
		ObjectFiles:     parsed.ObjectFiles,
		CompilerFlags:   parsed.CompilerFlags,
		IncludeDirs:     parsed.IncludeDirs,
		Defines:         parsed.Defines,
		Libraries:       parsed.Libraries,
		LibraryDirs:     parsed.LibraryDirs,
		LinkerFlags:     parsed.LinkerFlags,
		OutputFile:      parsed.OutputFile,
		IsLinkJob:       parsed.Mode == ModeLink,
		IsSharedLibrary: parsed.IsSharedLibrary,
		Timeout:         5 * time.Minute,
		WorkDir:         workDir,
	}
	return jsc.createJob(ctx)
}
