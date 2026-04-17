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

// TestCreateJobDataForFile_BasicFile tests creating JobData for a simple file
func TestCreateJobDataForFile_BasicFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("hello world")

	err := os.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err, "failed to write test file")

	stager := NewJobDataStager(tmpDir)
	jobData, err := stager.CreateJobDataForFile(context.Background(), "test.txt", JobDataModeReference)
	require.NoError(t, err, "CreateJobDataForFile failed")

	// Verify basic properties
	assert.Equal(t, v1.JobData_DATA_TYPE_FILE, jobData.Type)

	// Verify content is empty (daemon will load it)
	fileData := jobData.GetFile()
	require.NotNil(t, fileData, "expected file data in JobData")
	assert.Nil(t, fileData.Content, "expected Content to be empty (nil), but got data")

	// Verify path
	assert.Equal(t, "test.txt", fileData.Path)

	// Verify mode
	assert.Equal(t, uint32(0644), fileData.Mode)

	// Verify hash is present
	require.NotNil(t, fileData.ContentHash, "expected ContentHash to be set")
	expectedHash := ComputeContentHash(testContent)
	assert.Equal(t, expectedHash, fileData.ContentHash.Value, "hash mismatch")

	// Verify size
	require.NotNil(t, jobData.Size)
	assert.Equal(t, float64(len(testContent)), jobData.Size.Count)
}

// TestCreateJobDataForFile_AbsolutePath tests handling absolute paths
func TestCreateJobDataForFile_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	absPath := filepath.Join(tmpDir, "subdir", "test.txt")
	err := os.MkdirAll(filepath.Dir(absPath), 0755)
	require.NoError(t, err)

	err = os.WriteFile(absPath, []byte("absolute path test"), 0755)
	require.NoError(t, err, "failed to write test file")

	stager := NewJobDataStager(tmpDir)
	jobData, err := stager.CreateJobDataForFile(context.Background(), absPath, JobDataModeReference)
	require.NoError(t, err, "CreateJobDataForFile with absolute path failed")

	fileData := jobData.GetFile()
	// Path should be relative to workDir
	expectedPath := filepath.Join("subdir", "test.txt")
	assert.Equal(t, expectedPath, fileData.Path)
}

// TestCreateJobDataForFile_LargeFile tests streaming hash without loading full content
func TestCreateJobDataForFile_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.bin")

	// Create a 10MB file
	largeContent := make([]byte, 10*1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	err := os.WriteFile(testFile, largeContent, 0644)
	require.NoError(t, err, "failed to write large test file")

	stager := NewJobDataStager(tmpDir)
	jobData, err := stager.CreateJobDataForFile(context.Background(), "large.bin", JobDataModeReference)
	require.NoError(t, err, "CreateJobDataForFile for large file failed")

	// Verify hash is computed correctly (streaming, not loading all content)
	fileData := jobData.GetFile()
	expectedHash := ComputeContentHash(largeContent)
	assert.Equal(t, expectedHash, fileData.ContentHash.Value, "hash mismatch for large file")

	// Content should still be empty
	assert.Nil(t, fileData.Content, "expected Content to be empty for large file")
}

// TestCreateJobDataForFile_NonExistent tests error handling for missing files
func TestCreateJobDataForFile_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	stager := NewJobDataStager(tmpDir)

	jobData, err := stager.CreateJobDataForFile(context.Background(), "nonexistent.txt", JobDataModeReference)

	assert.Error(t, err, "expected error for non-existent file")
	assert.Nil(t, jobData, "expected nil JobData when error occurs")
}

// TestCreateJobDataForFile_DifferentPermissions tests various file permissions
func TestCreateJobDataForFile_DifferentPermissions(t *testing.T) {
	testCases := []struct {
		name string
		mode os.FileMode
	}{
		{"readable_writable", 0644},
		{"executable", 0755},
		{"read_only", 0444},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test.txt")

			err := os.WriteFile(testFile, []byte("test"), tc.mode)
			require.NoError(t, err, "failed to write test file")

			stager := NewJobDataStager(tmpDir)
			jobData, err := stager.CreateJobDataForFile(context.Background(), "test.txt", JobDataModeReference)
			require.NoError(t, err, "CreateJobDataForFile failed")

			fileData := jobData.GetFile()
			// On some systems, umask may affect actual permissions
			// Just verify that we captured what the file actually has
			actualFileInfo, _ := os.Stat(testFile)
			expectedMode := uint32(actualFileInfo.Mode())
			assert.Equal(t, expectedMode, fileData.Mode)
		})
	}
}

// TestCreateJobDataListForFiles tests batch file processing
func TestCreateJobDataListForFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple test files
	files := []struct {
		path    string
		content []byte
	}{
		{"file1.txt", []byte("content1")},
		{"subdir/file2.txt", []byte("content2")},
		{"file3.bin", []byte("binary\x00\x01\x02")},
	}

	for _, f := range files {
		fullPath := filepath.Join(tmpDir, f.path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)

		err = os.WriteFile(fullPath, f.content, 0644)
		require.NoError(t, err, "failed to write test file")
	}

	stager := NewJobDataStager(tmpDir)
	paths := []string{"file1.txt", "subdir/file2.txt", "file3.bin"}
	jobDataList, err := stager.CreateJobDataListForFiles(context.Background(), paths, JobDataModeReference)
	require.NoError(t, err, "CreateJobDataListForFiles failed")

	require.Len(t, jobDataList, len(files))

	// Verify each JobData
	for i, f := range files {
		fileData := jobDataList[i].GetFile()
		expectedHash := ComputeContentHash(f.content)
		assert.Equal(t, expectedHash, fileData.ContentHash.Value, "file %d: hash mismatch", i)
	}
}

// TestCreateJobDataListForFiles_PartialFailure tests error handling in batch processing
func TestCreateJobDataListForFiles_PartialFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only first file
	err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err, "failed to write first file")

	stager := NewJobDataStager(tmpDir)
	paths := []string{"file1.txt", "nonexistent.txt"}
	jobDataList, err := stager.CreateJobDataListForFiles(context.Background(), paths, JobDataModeReference)

	assert.Error(t, err, "expected error due to missing file")
	assert.Nil(t, jobDataList, "expected nil JobDataList when error occurs")
}

// TestWriteJobDataToFile_WriteNewFile tests writing new file with content
func TestWriteJobDataToFile_WriteNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	content := []byte("test output content")
	hash := ComputeContentHash(content)

	jobData := &v1.JobData{
		Id:   hash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "output.txt",
				Mode:    0644,
				Content: content,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     hash,
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.WriteJobDataToFile(context.Background(), jobData, outputDir)
	require.NoError(t, err, "WriteJobDataToFile failed")

	// Verify file was written
	writtenPath := filepath.Join(outputDir, "output.txt")
	writtenContent, err := os.ReadFile(writtenPath)
	require.NoError(t, err, "failed to read written file")
	assert.Equal(t, content, writtenContent)

	// Verify file permissions
	fileInfo, err := os.Stat(writtenPath)
	require.NoError(t, err, "failed to stat written file")
	assert.Equal(t, os.FileMode(0644), fileInfo.Mode())
}

// TestWriteJobDataToFile_SkipIfExists tests that existing files are not re-written
func TestWriteJobDataToFile_SkipIfExists(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := tmpDir

	// Pre-create output file with marker content
	outputFile := filepath.Join(outputDir, "output.txt")
	markerContent := []byte("ORIGINAL CONTENT")
	if err := os.WriteFile(outputFile, markerContent, 0644); err != nil {
		t.Fatalf("failed to create pre-existing file: %v", err)
	}
	markerHash := ComputeContentHash(markerContent)

	// Create JobData with different content and same hash (simulating runtime already wrote it)
	jobData := &v1.JobData{
		Id:   markerHash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "output.txt",
				Mode:    0644,
				Content: nil, // Content empty - already on disk
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     markerHash,
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.WriteJobDataToFile(context.Background(), jobData, outputDir)
	require.NoError(t, err, "WriteJobDataToFile failed")

	// Verify file still has original content (not re-written)
	finalContent, err := os.ReadFile(outputFile)
	require.NoError(t, err, "failed to read final file")
	assert.Equal(t, markerContent, finalContent, "file was re-written")
}

// TestWriteJobDataToFile_HashVerification tests hash verification on existing files
func TestWriteJobDataToFile_HashVerification(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := tmpDir

	// Create a test file
	content := []byte("correct content")
	outputFile := filepath.Join(outputDir, "output.txt")
	err := os.WriteFile(outputFile, content, 0644)
	require.NoError(t, err, "failed to create test file")
	correctHash := ComputeContentHash(content)

	// JobData with correct hash
	jobData := &v1.JobData{
		Id:   correctHash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "output.txt",
				Mode:    0644,
				Content: nil,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     correctHash,
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err = stager.WriteJobDataToFile(context.Background(), jobData, outputDir)
	assert.NoError(t, err, "WriteJobDataToFile should succeed with correct hash")
}

// TestWriteJobDataToFile_HashMismatch tests error on hash mismatch
func TestWriteJobDataToFile_HashMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := tmpDir

	// Create a test file
	content := []byte("actual content")
	outputFile := filepath.Join(outputDir, "output.txt")
	if err := os.WriteFile(outputFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// JobData with wrong hash
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	jobData := &v1.JobData{
		Id:   wrongHash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "output.txt",
				Mode:    0644,
				Content: nil,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     wrongHash,
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.WriteJobDataToFile(context.Background(), jobData, outputDir)

	assert.Error(t, err, "expected error due to hash mismatch")
}

// TestWriteJobDataToFile_CreateDirectories tests automatic directory creation
func TestWriteJobDataToFile_CreateDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	content := []byte("nested file content")
	hash := ComputeContentHash(content)

	jobData := &v1.JobData{
		Id:   hash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    filepath.Join("deeply", "nested", "dir", "file.txt"),
				Mode:    0644,
				Content: content,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     hash,
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.WriteJobDataToFile(context.Background(), jobData, outputDir)
	require.NoError(t, err, "WriteJobDataToFile failed")

	// Verify file exists in nested directory
	writtenPath := filepath.Join(outputDir, "deeply", "nested", "dir", "file.txt")
	_, err = os.Stat(writtenPath)
	require.NoError(t, err, "nested file not created")
}

// TestWriteJobDataToFile_AbsolutePath tests that absolute paths are rejected
func TestWriteJobDataToFile_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	content := []byte("absolute path test")
	hash := ComputeContentHash(content)
	absFilePath := filepath.Join(outputDir, "absolute.txt")

	jobData := &v1.JobData{
		Id:   hash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    absFilePath, // Absolute path
				Mode:    0644,
				Content: content,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     hash,
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.WriteJobDataToFile(context.Background(), jobData, outputDir)
	// Now we expect this to FAIL because absolute paths are not allowed
	require.Error(t, err, "WriteJobDataToFile should reject absolute paths")
	assert.Contains(t, err.Error(), "invalid job data")
	assert.Contains(t, err.Error(), "must be relative")
}

// TestWriteJobDataToFile_EmptyFile tests handling empty files
func TestWriteJobDataToFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	// Empty content
	hash := ComputeContentHash([]byte{})

	jobData := &v1.JobData{
		Id:   hash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "empty.txt",
				Mode:    0644,
				Content: []byte{},
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     hash,
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.WriteJobDataToFile(context.Background(), jobData, outputDir)
	require.NoError(t, err, "WriteJobDataToFile for empty file failed")

	// Verify file was created
	writtenPath := filepath.Join(outputDir, "empty.txt")
	fileInfo, err := os.Stat(writtenPath)
	require.NoError(t, err, "empty file not created")
	assert.Equal(t, int64(0), fileInfo.Size())
}

// TestWriteJobDataToFile_BinaryContent tests handling binary data
func TestWriteJobDataToFile_BinaryContent(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	// Binary content with null bytes and special characters
	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD, 'h', 'i', 0x00}
	hash := ComputeContentHash(binaryContent)

	jobData := &v1.JobData{
		Id:   hash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "binary.bin",
				Mode:    0644,
				Content: binaryContent,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     hash,
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.WriteJobDataToFile(context.Background(), jobData, outputDir)
	require.NoError(t, err, "WriteJobDataToFile for binary file failed")

	// Verify binary content was written correctly
	writtenPath := filepath.Join(outputDir, "binary.bin")
	writtenContent, err := os.ReadFile(writtenPath)
	require.NoError(t, err, "failed to read binary file")
	assert.Equal(t, binaryContent, writtenContent, "binary content mismatch")
}

// TestWriteJobDataListToFiles tests batch writing with various scenarios
func TestWriteJobDataListToFiles(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	// Create mix of scenarios
	file1Content := []byte("file1 content")
	file1Hash := ComputeContentHash(file1Content)

	file2Content := []byte("file2 different content")
	file2Hash := ComputeContentHash(file2Content)

	file3Content := []byte{0xFF, 0xFE, 0xFD}
	file3Hash := ComputeContentHash(file3Content)

	jobDataList := []*v1.JobData{
		{
			Id:   file1Hash,
			Type: v1.JobData_DATA_TYPE_FILE,
			Data: &v1.JobData_File{
				File: &v1.FileJobData{
					Path:    "file1.txt",
					Mode:    0644,
					Content: file1Content,
					ContentHash: &v1.Hash{
						Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
						Value:     file1Hash,
					},
				},
			},
		},
		{
			Id:   file2Hash,
			Type: v1.JobData_DATA_TYPE_FILE,
			Data: &v1.JobData_File{
				File: &v1.FileJobData{
					Path:    filepath.Join("subdir", "file2.txt"),
					Mode:    0755,
					Content: file2Content,
					ContentHash: &v1.Hash{
						Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
						Value:     file2Hash,
					},
				},
			},
		},
		{
			Id:   file3Hash,
			Type: v1.JobData_DATA_TYPE_FILE,
			Data: &v1.JobData_File{
				File: &v1.FileJobData{
					Path:    "binary.bin",
					Mode:    0644,
					Content: file3Content,
					ContentHash: &v1.Hash{
						Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
						Value:     file3Hash,
					},
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.WriteJobDataListToFiles(context.Background(), jobDataList, outputDir)
	require.NoError(t, err, "WriteJobDataListToFiles failed")

	// Verify all files were written
	files := []struct {
		path    string
		content []byte
	}{
		{"file1.txt", file1Content},
		{filepath.Join("subdir", "file2.txt"), file2Content},
		{"binary.bin", file3Content},
	}

	for _, f := range files {
		fullPath := filepath.Join(outputDir, f.path)
		writtenContent, err := os.ReadFile(fullPath)
		require.NoError(t, err, "failed to read %s", f.path)
		assert.Equal(t, f.content, writtenContent, "content mismatch for %s", f.path)
	}
}

// TestWriteJobDataListToFiles_MixedExistingAndNew tests batch processing with pre-existing files
func TestWriteJobDataListToFiles_MixedExistingAndNew(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	// Pre-create one file (simulating runtime already wrote it)
	existingContent := []byte("pre-existing file")
	existingPath := filepath.Join(outputDir, "existing.txt")
	if err := os.WriteFile(existingPath, existingContent, 0644); err != nil {
		t.Fatalf("failed to create pre-existing file: %v", err)
	}
	existingHash := ComputeContentHash(existingContent)

	// New file to be written
	newContent := []byte("newly written file")
	newHash := ComputeContentHash(newContent)

	jobDataList := []*v1.JobData{
		{
			Id:   existingHash,
			Type: v1.JobData_DATA_TYPE_FILE,
			Data: &v1.JobData_File{
				File: &v1.FileJobData{
					Path:    "existing.txt",
					Mode:    0644,
					Content: nil, // Empty - already on disk
					ContentHash: &v1.Hash{
						Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
						Value:     existingHash,
					},
				},
			},
		},
		{
			Id:   newHash,
			Type: v1.JobData_DATA_TYPE_FILE,
			Data: &v1.JobData_File{
				File: &v1.FileJobData{
					Path:    "new.txt",
					Mode:    0644,
					Content: newContent,
					ContentHash: &v1.Hash{
						Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
						Value:     newHash,
					},
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.WriteJobDataListToFiles(context.Background(), jobDataList, outputDir)
	require.NoError(t, err, "WriteJobDataListToFiles failed")

	// Verify existing file wasn't changed
	finalExistingContent, _ := os.ReadFile(existingPath)
	assert.Equal(t, existingContent, finalExistingContent, "existing file was modified")

	// Verify new file was created
	newPath := filepath.Join(outputDir, "new.txt")
	finalNewContent, _ := os.ReadFile(newPath)
	assert.Equal(t, newContent, finalNewContent, "new file content mismatch")
}

// TestComputeFileHash tests file hashing
func TestComputeFileHash(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("hash test content")

	err := os.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err, "failed to create test file")

	hash, err := ComputeFileHash(testFile)
	require.NoError(t, err, "ComputeFileHash failed")

	expectedHash := ComputeContentHash(testContent)
	assert.Equal(t, expectedHash, hash)
}

// TestComputeFileHash_NonExistent tests error handling for missing files
func TestComputeFileHash_NonExistent(t *testing.T) {
	hash, err := ComputeFileHash("/nonexistent/file.txt")

	assert.Error(t, err, "expected error for non-existent file")
	assert.Empty(t, hash, "expected empty hash string on error")
}

// TestComputeContentHash tests content hashing
func TestComputeContentHash(t *testing.T) {
	testCases := []struct {
		name     string
		content  []byte
		expected string // First 8 chars for quick verification
	}{
		{
			name:     "empty",
			content:  []byte{},
			expected: "e3b0c442", // SHA256 of empty data
		},
		{
			name:     "simple text",
			content:  []byte("hello"),
			expected: "2cf24dba", // SHA256 of "hello"
		},
		{
			name:     "binary",
			content:  []byte{0x00, 0x01, 0x02, 0xFF},
			expected: "", // Don't hardcode - just verify it computes
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash := ComputeContentHash(tc.content)

			assert.NotEmpty(t, hash, "expected non-empty hash")
			assert.Equal(t, 64, len(hash), "expected 64 character hash")

			if tc.expected != "" {
				assert.Equal(t, tc.expected, hash[:8], "hash mismatch")
			}
		})
	}
}

// TestIntegration_InputToOutput tests complete round trip: create input, write output
func TestIntegration_InputToOutput(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := tmpDir
	outputDir := filepath.Join(tmpDir, "output")

	// Step 1: Create input files
	inputContent := []byte("test input content")
	inputFile := filepath.Join(inputDir, "input.txt")
	if err := os.WriteFile(inputFile, inputContent, 0644); err != nil {
		t.Fatalf("failed to create input file: %v", err)
	}

	// Step 2: Stage input as JobData
	stager := NewJobDataStager(inputDir)
	_, err := stager.CreateJobDataForFile(context.Background(), "input.txt", JobDataModeReference)
	if err != nil {
		t.Fatalf("CreateJobDataForFile failed: %v", err)
	}

	// Step 3: Simulate daemon/runtime processing:
	// Create output JobData with content
	outputContent := []byte("processed output")
	outputHash := ComputeContentHash(outputContent)

	outputJobData := &v1.JobData{
		Id:   outputHash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "output.txt",
				Mode:    0755,
				Content: outputContent,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     outputHash,
				},
			},
		},
	}

	// Step 4: Write output
	err = stager.WriteJobDataToFile(context.Background(), outputJobData, outputDir)
	require.NoError(t, err, "WriteJobDataToFile failed")

	// Step 5: Verify output
	outputPath := filepath.Join(outputDir, "output.txt")
	writtenContent, err := os.ReadFile(outputPath)
	require.NoError(t, err, "failed to read output file")
	assert.Equal(t, outputContent, writtenContent, "output content mismatch")

	// Verify permissions
	fileInfo, err := os.Stat(outputPath)
	require.NoError(t, err, "failed to stat output file")
	assert.Equal(t, os.FileMode(0755), fileInfo.Mode())
}

// ============================================================================
// Verification Tests
// ============================================================================

// TestVerifyJobData_FileSavedMode_Success tests verifying a saved file with correct hash
func TestVerifyJobData_FileSavedMode_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("test content")
	require.NoError(t, os.WriteFile(testFile, testContent, 0644))

	hash := ComputeContentHash(testContent)
	jobData := &v1.JobData{
		Id:   hash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path: "test.txt",
				Mode: 0644,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     hash,
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobData(context.Background(), jobData, tmpDir, VerificationModeSaved)
	require.NoError(t, err, "verification should succeed for saved file with correct hash")
}

// TestVerifyJobData_FileSavedMode_NotFound tests error when file doesn't exist
func TestVerifyJobData_FileSavedMode_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	jobData := &v1.JobData{
		Id:   "fakehash",
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "missing.txt",
				Mode:    0644,
				Content: nil,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     "fakehash",
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobData(context.Background(), jobData, tmpDir, VerificationModeSaved)
	require.Error(t, err, "verification should fail for missing file")
	assert.Contains(t, err.Error(), "file not found")
}

// TestVerifyJobData_FileSavedMode_HashMismatch tests error on hash mismatch
func TestVerifyJobData_FileSavedMode_HashMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("actual content")
	require.NoError(t, os.WriteFile(testFile, testContent, 0644))

	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	jobData := &v1.JobData{
		Id:   wrongHash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path: "test.txt",
				Mode: 0644,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     wrongHash,
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobData(context.Background(), jobData, tmpDir, VerificationModeSaved)
	require.Error(t, err, "verification should fail on hash mismatch")
	assert.Contains(t, err.Error(), "file hash mismatch")
}

// TestVerifyJobData_IntegrityMode_WithContent tests integrity verification with in-memory content
func TestVerifyJobData_IntegrityMode_WithContent(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte("in-memory content")
	hash := ComputeContentHash(content)

	jobData := &v1.JobData{
		Id:   hash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "test.txt",
				Mode:    0644,
				Content: content,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     hash,
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobData(context.Background(), jobData, tmpDir, VerificationModeIntegrity)
	require.NoError(t, err, "verification should succeed for content-based verification")
}

// TestVerifyJobData_IntegrityMode_WithContentHashMismatch tests content hash verification failure
func TestVerifyJobData_IntegrityMode_WithContentHashMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte("in-memory content")
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"

	jobData := &v1.JobData{
		Id:   wrongHash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "test.txt",
				Mode:    0644,
				Content: content,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     wrongHash,
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobData(context.Background(), jobData, tmpDir, VerificationModeIntegrity)
	require.Error(t, err, "verification should fail on content hash mismatch")
	assert.Contains(t, err.Error(), "content hash mismatch")
}

// TestVerifyJobData_IntegrityMode_WithReference tests integrity verification with file reference
func TestVerifyJobData_IntegrityMode_WithReference(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("referenced content")
	require.NoError(t, os.WriteFile(testFile, testContent, 0644))

	hash := ComputeContentHash(testContent)
	jobData := &v1.JobData{
		Id:   hash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "test.txt",
				Mode:    0644,
				Content: nil, // Reference, no content
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     hash,
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobData(context.Background(), jobData, tmpDir, VerificationModeIntegrity)
	require.NoError(t, err, "verification should succeed for reference-based verification")
}

// TestVerifyJobData_IntegrityMode_ReferenceNotFound tests error when referenced file doesn't exist
func TestVerifyJobData_IntegrityMode_ReferenceNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	jobData := &v1.JobData{
		Id:   "fakehash",
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    "missing.txt",
				Mode:    0644,
				Content: nil,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     "fakehash",
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobData(context.Background(), jobData, tmpDir, VerificationModeIntegrity)
	require.Error(t, err, "verification should fail for missing referenced file")
	assert.Contains(t, err.Error(), "file not found")
}

// TestVerifyJobDataList_Success tests batch verification of multiple files
func TestVerifyJobDataList_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple test files
	files := []struct {
		name    string
		content []byte
	}{
		{"file1.txt", []byte("content1")},
		{"file2.txt", []byte("content2")},
		{"file3.txt", []byte("content3")},
	}

	var jobDataList []*v1.JobData
	for _, f := range files {
		filePath := filepath.Join(tmpDir, f.name)
		require.NoError(t, os.WriteFile(filePath, f.content, 0644))

		hash := ComputeContentHash(f.content)
		jobData := &v1.JobData{
			Id:   hash,
			Type: v1.JobData_DATA_TYPE_FILE,
			Data: &v1.JobData_File{
				File: &v1.FileJobData{
					Path: f.name,
					Mode: 0644,
					ContentHash: &v1.Hash{
						Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
						Value:     hash,
					},
				},
			},
		}
		jobDataList = append(jobDataList, jobData)
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobDataList(context.Background(), jobDataList, tmpDir, VerificationModeSaved)
	require.NoError(t, err, "batch verification should succeed")
}

// TestVerifyJobDataList_PartialFailure tests batch verification stops on first error
func TestVerifyJobDataList_PartialFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only first file
	filePath := filepath.Join(tmpDir, "file1.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content1"), 0644))

	hash1 := ComputeContentHash([]byte("content1"))
	jobData1 := &v1.JobData{
		Id:   hash1,
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path: "file1.txt",
				Mode: 0644,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     hash1,
				},
			},
		},
	}

	// Second file doesn't exist
	jobData2 := &v1.JobData{
		Id:   "fakehash",
		Type: v1.JobData_DATA_TYPE_FILE,
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path: "missing.txt",
				Mode: 0644,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     "fakehash",
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobDataList(context.Background(), []*v1.JobData{jobData1, jobData2}, tmpDir, VerificationModeSaved)
	require.Error(t, err, "batch verification should fail on missing file")
	assert.Contains(t, err.Error(), "file not found")
}

// TestVerifyJobResultOutputs_Success tests verification of job result outputs
func TestVerifyJobResultOutputs_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create output file
	outFile := filepath.Join(tmpDir, "output.o")
	outContent := []byte("object file content")
	require.NoError(t, os.WriteFile(outFile, outContent, 0644))

	outHash := ComputeContentHash(outContent)

	jobResult := &v1.JobResult{
		ExitCode: 0,
		Outputs: []*v1.JobData{
			{
				Id:   outHash,
				Type: v1.JobData_DATA_TYPE_FILE,
				Data: &v1.JobData_File{
					File: &v1.FileJobData{
						Path: "output.o",
						Mode: 0644,
						ContentHash: &v1.Hash{
							Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
							Value:     outHash,
						},
					},
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobResultOutputs(context.Background(), jobResult, tmpDir, VerificationModeSaved)
	require.NoError(t, err, "job result output verification should succeed")
}

// TestVerifyJobResultOutputs_NoOutputs tests verification with empty outputs
func TestVerifyJobResultOutputs_NoOutputs(t *testing.T) {
	tmpDir := t.TempDir()

	jobResult := &v1.JobResult{
		ExitCode: 0,
		Outputs:  nil,
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobResultOutputs(context.Background(), jobResult, tmpDir, VerificationModeSaved)
	require.NoError(t, err, "verification with no outputs should succeed")
}

// TestVerifyJobResultOutputs_Nil tests error on nil job result
func TestVerifyJobResultOutputs_Nil(t *testing.T) {
	stager := NewJobDataStager(".")
	err := stager.VerifyJobResultOutputs(context.Background(), nil, ".", VerificationModeSaved)
	require.Error(t, err, "verification should fail for nil job result")
	assert.Contains(t, err.Error(), "job result is nil")
}

// TestVerifyJobInputs_Success tests verification of job inputs
func TestVerifyJobInputs_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input files
	input1Path := filepath.Join(tmpDir, "input1.c")
	input1Content := []byte("#include <stdio.h>")
	require.NoError(t, os.WriteFile(input1Path, input1Content, 0644))

	input2Path := filepath.Join(tmpDir, "input2.c")
	input2Content := []byte("int main() {}")
	require.NoError(t, os.WriteFile(input2Path, input2Content, 0644))

	job := &v1.Job{
		Id:  "test-job",
		Cwd: tmpDir,
		Inputs: []*v1.JobData{
			{
				Id:   ComputeContentHash(input1Content),
				Type: v1.JobData_DATA_TYPE_FILE,
				Data: &v1.JobData_File{
					File: &v1.FileJobData{
						Path: "input1.c",
						Mode: 0644,
						ContentHash: &v1.Hash{
							Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
							Value:     ComputeContentHash(input1Content),
						},
					},
				},
			},
			{
				Id:   ComputeContentHash(input2Content),
				Type: v1.JobData_DATA_TYPE_FILE,
				Data: &v1.JobData_File{
					File: &v1.FileJobData{
						Path: "input2.c",
						Mode: 0644,
						ContentHash: &v1.Hash{
							Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
							Value:     ComputeContentHash(input2Content),
						},
					},
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobInputs(context.Background(), job, VerificationModeSaved)
	require.NoError(t, err, "job input verification should succeed")
}

// TestVerifyJobInputs_MissingInput tests error when input file doesn't exist
func TestVerifyJobInputs_MissingInput(t *testing.T) {
	tmpDir := t.TempDir()

	job := &v1.Job{
		Id:  "test-job",
		Cwd: tmpDir,
		Inputs: []*v1.JobData{
			{
				Id:   "fakehash",
				Type: v1.JobData_DATA_TYPE_FILE,
				Data: &v1.JobData_File{
					File: &v1.FileJobData{
						Path: "missing.c",
						Mode: 0644,
						ContentHash: &v1.Hash{
							Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
							Value:     "fakehash",
						},
					},
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobInputs(context.Background(), job, VerificationModeSaved)
	require.Error(t, err, "job input verification should fail for missing file")
	assert.Contains(t, err.Error(), "file not found")
}

// TestVerifyJobInputs_HashMismatch tests error when input hash doesn't match
func TestVerifyJobInputs_HashMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	inputPath := filepath.Join(tmpDir, "input.c")
	inputContent := []byte("#include <stdio.h>")
	require.NoError(t, os.WriteFile(inputPath, inputContent, 0644))

	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"

	job := &v1.Job{
		Id:  "test-job",
		Cwd: tmpDir,
		Inputs: []*v1.JobData{
			{
				Id:   wrongHash,
				Type: v1.JobData_DATA_TYPE_FILE,
				Data: &v1.JobData_File{
					File: &v1.FileJobData{
						Path: "input.c",
						Mode: 0644,
						ContentHash: &v1.Hash{
							Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
							Value:     wrongHash,
						},
					},
				},
			},
		},
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobInputs(context.Background(), job, VerificationModeSaved)
	require.Error(t, err, "job input verification should fail on hash mismatch")
	assert.Contains(t, err.Error(), "file hash mismatch")
}

// TestVerifyJobInputs_NoInputs tests verification with no inputs
func TestVerifyJobInputs_NoInputs(t *testing.T) {
	tmpDir := t.TempDir()

	job := &v1.Job{
		Id:     "test-job",
		Cwd:    tmpDir,
		Inputs: nil,
	}

	stager := NewJobDataStager(tmpDir)
	err := stager.VerifyJobInputs(context.Background(), job, VerificationModeSaved)
	require.NoError(t, err, "verification with no inputs should succeed")
}

// TestVerifyJobInputs_Nil tests error on nil job
func TestVerifyJobInputs_Nil(t *testing.T) {
	stager := NewJobDataStager(".")
	err := stager.VerifyJobInputs(context.Background(), nil, VerificationModeSaved)
	require.Error(t, err, "verification should fail for nil job")
	assert.Contains(t, err.Error(), "job is nil")
}

// TestVerifyJobData_Nil tests error on nil job data
func TestVerifyJobData_Nil(t *testing.T) {
	stager := NewJobDataStager(".")
	err := stager.VerifyJobData(context.Background(), nil, ".", VerificationModeSaved)
	require.Error(t, err, "verification should fail for nil job data")
	assert.Contains(t, err.Error(), "job data is nil")
}

// TestVerifyJobDataList_Empty tests verification with empty list
func TestVerifyJobDataList_Empty(t *testing.T) {
	stager := NewJobDataStager(".")
	err := stager.VerifyJobDataList(context.Background(), nil, ".", VerificationModeSaved)
	require.NoError(t, err, "verification with empty list should succeed")

	err = stager.VerifyJobDataList(context.Background(), []*v1.JobData{}, ".", VerificationModeSaved)
	require.NoError(t, err, "verification with empty slice should succeed")
}
