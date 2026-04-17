package gcc_common

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJobCreation_InputDataIdsNotPopulated tests that InputDataIds is not populated (should use Inputs instead)
func TestJobCreation_InputDataIdsNotPopulated(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test source file
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0755)
	srcFile := filepath.Join(srcDir, "test.c")
	err := os.WriteFile(srcFile, []byte("int main() { return 0; }"), 0644)
	require.NoError(t, err)

	// Create test output directory
	outputDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(outputDir, 0755)

	// Create JobSubmissionContext with absolute paths (as CMake would provide)
	jsc := &JobSubmissionContext{
		WorkDir:     outputDir,
		SourceFiles: []string{srcFile}, // Absolute path from CMake
		OutputFile:  "test.o",
		Timeout:     30 * time.Second,
		Runtime: &v1.Runtime{
			Id: "test-runtime",
		},
	}

	// Create job
	job, err := jsc.createJob(context.Background())
	require.NoError(t, err, "createJob failed")

	// Verify InputDataIds is not populated
	assert.Empty(t, job.InputDataIds, "InputDataIds should be empty (should use Inputs instead)")

	// Verify Inputs are populated with relative paths
	assert.NotEmpty(t, job.Inputs, "Inputs should be populated")
	require.Len(t, job.Inputs, 1, "should have one input file")

	// Verify the input path is relative (not absolute)
	inputFileData := job.Inputs[0].GetFile()
	require.NotNil(t, inputFileData, "input should be a file")
	assert.False(t, filepath.IsAbs(inputFileData.Path), "input path should be relative, got: %s", inputFileData.Path)
}

// TestJobCreation_OutputPathRelative tests that output paths are relative
func TestJobCreation_OutputPathRelative(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0755)

	srcFile := filepath.Join(srcDir, "test.c")
	err := os.WriteFile(srcFile, []byte("int main() { return 0; }"), 0644)
	require.NoError(t, err)

	outputDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(outputDir, 0755)

	jsc := &JobSubmissionContext{
		WorkDir:     outputDir,
		SourceFiles: []string{srcFile},
		OutputFile:  "test.o",
		Timeout:     30 * time.Second,
		Runtime: &v1.Runtime{
			Id: "test-runtime",
		},
	}

	job, err := jsc.createJob(context.Background())
	require.NoError(t, err, "createJob failed")

	// Verify output path is relative
	assert.NotEmpty(t, job.Outputs, "should have outputs")
	require.Len(t, job.Outputs, 1, "should have one output")

	outputFileData := job.Outputs[0].GetFile()
	require.NotNil(t, outputFileData, "output should be a file")
	assert.False(t, filepath.IsAbs(outputFileData.Path), "output path should be relative, got: %s", outputFileData.Path)
	assert.Equal(t, "test.o", outputFileData.Path)
}
