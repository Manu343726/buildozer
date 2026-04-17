package staging

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateJobDataPath_RelativeFile tests that relative paths are accepted
func TestValidateJobDataPath_RelativeFile(t *testing.T) {
	jobData := &v1.JobData{
		Id:   "test-hash",
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path: "relative/path/file.txt",
			},
		},
	}

	err := validateJobDataPath(jobData)
	require.NoError(t, err, "relative path should be accepted")
}

// TestValidateJobDataPath_AbsoluteFile tests that absolute paths are rejected
func TestValidateJobDataPath_AbsoluteFile(t *testing.T) {
	jobData := &v1.JobData{
		Id:   "test-hash",
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path: "/absolute/path/file.txt",
			},
		},
	}

	err := validateJobDataPath(jobData)
	require.Error(t, err, "absolute path should be rejected")
	assert.Contains(t, err.Error(), "must be relative")
	assert.Contains(t, err.Error(), "/absolute/path/file.txt")
}

// TestValidateJobDataPath_RelativeDirectory tests that relative directory paths are accepted
func TestValidateJobDataPath_RelativeDirectory(t *testing.T) {
	jobData := &v1.JobData{
		Id:   "test-hash",
		Type: v1.JobData_DATA_TYPE_DIRECTORY,
		Data: &v1.JobData_Directory{
			Directory: &v1.DirectoryJobData{
				Path: "relative/dir",
			},
		},
	}

	err := validateJobDataPath(jobData)
	require.NoError(t, err, "relative directory path should be accepted")
}

// TestValidateJobDataPath_AbsoluteDirectory tests that absolute directory paths are rejected
func TestValidateJobDataPath_AbsoluteDirectory(t *testing.T) {
	jobData := &v1.JobData{
		Id:   "test-hash",
		Type: v1.JobData_DATA_TYPE_DIRECTORY,
		Data: &v1.JobData_Directory{
			Directory: &v1.DirectoryJobData{
				Path: "/absolute/dir",
			},
		},
	}

	err := validateJobDataPath(jobData)
	require.Error(t, err, "absolute directory path should be rejected")
	assert.Contains(t, err.Error(), "must be relative")
	assert.Contains(t, err.Error(), "/absolute/dir")
}

// TestValidateJobDataPath_Nil tests that nil job data is handled gracefully
func TestValidateJobDataPath_Nil(t *testing.T) {
	err := validateJobDataPath(nil)
	require.NoError(t, err, "nil job data should be accepted")
}

// TestValidateJobDataPathList_AllRelative tests that lists with all relative paths are accepted
func TestValidateJobDataPathList_AllRelative(t *testing.T) {
	jobDataList := []*v1.JobData{
		{
			Id:   "hash1",
			Type: v1.JobData_DATA_TYPE_FILE,
			Data: &v1.JobData_File{
				File: &v1.FileJobData{Path: "file1.txt"},
			},
		},
		{
			Id:   "hash2",
			Type: v1.JobData_DATA_TYPE_FILE,
			Data: &v1.JobData_File{
				File: &v1.FileJobData{Path: "subdir/file2.txt"},
			},
		},
	}

	err := validateJobDataPathList(jobDataList)
	require.NoError(t, err, "all relative paths should be accepted")
}

// TestValidateJobDataPathList_WithAbsolute tests that lists with absolute paths are rejected
func TestValidateJobDataPathList_WithAbsolute(t *testing.T) {
	jobDataList := []*v1.JobData{
		{
			Id:   "hash1",
			Type: v1.JobData_DATA_TYPE_FILE,
			Data: &v1.JobData_File{
				File: &v1.FileJobData{Path: "file1.txt"},
			},
		},
		{
			Id:   "hash2",
			Type: v1.JobData_DATA_TYPE_FILE,
			Data: &v1.JobData_File{
				File: &v1.FileJobData{Path: "/absolute/path/file2.txt"},
			},
		},
	}

	err := validateJobDataPathList(jobDataList)
	require.Error(t, err, "list with absolute path should be rejected")
	assert.Contains(t, err.Error(), "index 1")
	assert.Contains(t, err.Error(), "must be relative")
}

// TestWriteJobDataToFile_RejectsAbsolutePath tests that WriteJobDataToFile rejects absolute paths
func TestWriteJobDataToFile_RejectsAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	stager := NewJobDataStager(tmpDir)

	jobData := &v1.JobData{
		Id:   "test-hash",
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "/absolute/path/file.txt",
				Mode:    0644,
				Content: []byte("test content"),
			},
		},
	}

	err := stager.WriteJobDataToFile(context.Background(), jobData, tmpDir)
	require.Error(t, err, "WriteJobDataToFile should reject absolute paths")
	assert.Contains(t, err.Error(), "invalid job data")
	assert.Contains(t, err.Error(), "must be relative")
}

// TestEmbedJobData_RejectsAbsolutePath tests that EmbedJobData rejects absolute paths
func TestEmbedJobData_RejectsAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	stager := NewJobDataStager(tmpDir)

	jobData := &v1.JobData{
		Id:   "test-hash",
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path: "/absolute/path/file.txt",
			},
		},
	}

	_, err := stager.EmbedJobData(context.Background(), jobData)
	require.Error(t, err, "EmbedJobData should reject absolute paths")
	assert.Contains(t, err.Error(), "invalid job data")
	assert.Contains(t, err.Error(), "must be relative")
}

// TestEmbedJobDataList_RejectsAbsolutePath tests that EmbedJobDataList rejects absolute paths
func TestEmbedJobDataList_RejectsAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	stager := NewJobDataStager(tmpDir)

	jobDataList := []*v1.JobData{
		{
			Id:   "hash1",
			Type: v1.JobData_DATA_TYPE_FILE,
			Data: &v1.JobData_File{
				File: &v1.FileJobData{Path: "file1.txt"},
			},
		},
		{
			Id:   "hash2",
			Type: v1.JobData_DATA_TYPE_FILE,
			Data: &v1.JobData_File{
				File: &v1.FileJobData{Path: "/absolute/path/file2.txt"},
			},
		},
	}

	_, err := stager.EmbedJobDataList(context.Background(), jobDataList)
	require.Error(t, err, "EmbedJobDataList should reject absolute paths")
	assert.Contains(t, err.Error(), "invalid job data list")
	assert.Contains(t, err.Error(), "index 1")
}

// TestMaterializeJobOutputs_RejectsAbsolutePath tests that MaterializeJobOutputs rejects absolute paths
func TestMaterializeJobOutputs_RejectsAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	stager := NewJobDataStager(tmpDir)

	job := &v1.Job{
		Id:  "test-job",
		Cwd: tmpDir,
		Outputs: []*v1.JobData{
			{
				Id:   "output-hash",
				Type: v1.JobData_DATA_TYPE_FILE,
				Data: &v1.JobData_File{
					File: &v1.FileJobData{
						Path: "/absolute/output.txt",
					},
				},
			},
		},
	}

	_, err := stager.MaterializeJobOutputs(context.Background(), job, nil, tmpDir)
	require.Error(t, err, "MaterializeJobOutputs should reject absolute paths in outputs")
	assert.Contains(t, err.Error(), "invalid job outputs")
	assert.Contains(t, err.Error(), "must be relative")
}

// TestWriteJobDataToFile_AcceptsRelativePath tests that WriteJobDataToFile accepts relative paths
func TestWriteJobDataToFile_AcceptsRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	stager := NewJobDataStager(tmpDir)

	testContent := []byte("test content")
	hash := ComputeContentHash(testContent)

	jobData := &v1.JobData{
		Id:   hash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "subdir/output.txt",
				Mode:    0644,
				Content: testContent,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     hash,
				},
			},
		},
	}

	err := stager.WriteJobDataToFile(context.Background(), jobData, tmpDir)
	require.NoError(t, err, "WriteJobDataToFile should accept relative paths")

	// Verify file was written
	outputFile := filepath.Join(tmpDir, "subdir", "output.txt")
	assert.FileExists(t, outputFile)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, testContent, content)
}
