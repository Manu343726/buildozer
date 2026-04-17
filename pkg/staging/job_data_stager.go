// Package staging provides utilities for staging job input/output data.
// This package is shared between drivers and daemon to ensure consistent file handling.
package staging

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// JobDataMode specifies how to handle file content in JobData
type JobDataMode int

const (
	// JobDataModeReference creates JobData with path + hash only (file on disk, no content loaded)
	// Used for inputs and unsandboxed outputs where files already exist on the system
	JobDataModeReference JobDataMode = iota
	// JobDataModeContent creates JobData with content + hash (file content loaded into memory)
	// Used for sandboxed/remote runtimes that need file content transferred
	JobDataModeContent
)

// JobDataStager handles reading input files and materializing output files
type JobDataStager struct {
	workDir string
}

// NewJobDataStager creates a new job data stager
func NewJobDataStager(workDir string) *JobDataStager {
	return &JobDataStager{workDir: workDir}
}

// validateJobDataPath checks that all file paths in JobData are relative, not absolute.
// This is critical to prevent security issues and ensure portable job execution.
// Returns an error if any file path is absolute.
func validateJobDataPath(jobData *v1.JobData) error {
	if jobData == nil {
		return nil
	}

	// Check reference paths
	if ref := jobData.GetReference(); ref != nil {
		if filepath.IsAbs(ref.Id) {
			return fmt.Errorf("job data reference path must be relative, got absolute path: %s", ref.Id)
		}
	}

	// Check file paths
	if fileData := jobData.GetFile(); fileData != nil {
		if filepath.IsAbs(fileData.Path) {
			return fmt.Errorf("job data file path must be relative, got absolute path: %s", fileData.Path)
		}
	}

	// Check directory paths
	if dirData := jobData.GetDirectory(); dirData != nil {
		if filepath.IsAbs(dirData.Path) {
			return fmt.Errorf("job data directory path must be relative, got absolute path: %s", dirData.Path)
		}
	}

	return nil
}

// validateJobDataPathList checks that all file paths in a list of JobData are relative.
// Returns an error if any JobData has an absolute path.
func validateJobDataPathList(jobDataList []*v1.JobData) error {
	for i, jobData := range jobDataList {
		if err := validateJobDataPath(jobData); err != nil {
			return fmt.Errorf("job data at index %d: %w", i, err)
		}
	}
	return nil
}

// CreateJobDataForFile creates a JobData message for a single file.
// The mode parameter controls how file content is handled:
// - JobDataModeReference: stores path + hash only (file on disk, no content loaded)
// - JobDataModeContent: stores content + hash (file content loaded into memory)
// In both cases, the file hash is computed and stored for validation.
func (s *JobDataStager) CreateJobDataForFile(ctx context.Context, filePath string, mode JobDataMode) (*v1.JobData, error) {
	// Resolve path relative to workDir if not absolute
	absPath := s.resolveAbsPath(filePath)

	// Get file info for permissions and size
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", absPath, err)
	}

	// Compute file hash
	hashStr, err := ComputeFileHash(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to hash file %s: %w", absPath, err)
	}

	// Convert absolute path to be relative to workDir for storage in JobData
	// This ensures the daemon and runtime can extract files maintaining relative path structure
	storagePath := filePath
	if filepath.IsAbs(absPath) {
		relPath, err := filepath.Rel(s.workDir, absPath)
		if err == nil {
			storagePath = relPath
		}
		// If Rel fails, keep the original filePath (which may be relative already)
	}

	var jobDataContent []byte
	if mode == JobDataModeContent {
		// Load file content into memory for sandboxed/remote runtimes
		jobDataContent, err = os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", absPath, err)
		}
	}

	// Create JobData with appropriate content handling based on mode
	return createFileJobDataProto(storagePath, fileInfo, hashStr, jobDataContent), nil
}

// CreateJobDataListForFiles creates JobData messages for multiple files with the specified mode.
// See CreateJobDataForFile for mode details.
func (s *JobDataStager) CreateJobDataListForFiles(ctx context.Context, filePaths []string, mode JobDataMode) ([]*v1.JobData, error) {
	var result []*v1.JobData

	for _, filePath := range filePaths {
		jobData, err := s.CreateJobDataForFile(ctx, filePath, mode)
		if err != nil {
			return nil, err
		}
		result = append(result, jobData)
	}

	return result, nil
}

// MaterializeJobOutputs collects and materializes output files based on job specification.
// This implements the unified output handling algorithm for both driver and daemon.
//
// Algorithm (based on content presence):
//   - Content filled: Output not yet written to disk. Materialize writes it and verifies hash.
//   - Content empty: Output already written to disk by runtime. Materialize verifies file exists and hash.
//
// Parameters:
//   - job: Job specification containing expected outputs
//   - jobResult: Result from execution (may contain outputs with content for sandboxed runtimes)
//   - workDir: Working directory for resolving relative paths
//
// Returns:
//   - Collected outputs as JobData with empty content (materialize happens first)
//   - Error if validation fails (missing expected outputs, hash mismatch, etc.)
func (s *JobDataStager) MaterializeJobOutputs(ctx context.Context, job *v1.Job, jobResult *v1.JobResult, workDir string) ([]*v1.JobData, error) {
	if len(job.Outputs) == 0 {
		return nil, nil
	}

	// Validate that all expected outputs have relative paths
	if err := validateJobDataPathList(job.Outputs); err != nil {
		return nil, fmt.Errorf("invalid job outputs: %w", err)
	}

	// Validate that result outputs (if any) have relative paths
	if jobResult != nil && len(jobResult.Outputs) > 0 {
		if err := validateJobDataPathList(jobResult.Outputs); err != nil {
			return nil, fmt.Errorf("invalid job result outputs: %w", err)
		}
	}

	var outputs []*v1.JobData

	// Use provided workDir or job's CWD
	dir := workDir
	if dir == "" {
		dir = job.Cwd
	}
	if dir == "" {
		dir = "."
	}

	// Build set of expected output paths for validation
	expectedPaths := make(map[string]bool)
	for _, output := range job.Outputs {
		if output == nil {
			continue
		}
		fileData, ok := output.Data.(*v1.JobData_File)
		if ok && fileData.File != nil {
			expectedPaths[fileData.File.Path] = true
		}
	}

	// Validate result outputs are in expected set (silently ignore unexpected)
	if jobResult != nil && len(jobResult.Outputs) > 0 {
		for _, resultOutput := range jobResult.Outputs {
			if resultOutput == nil {
				continue
			}
			if resultFileData, ok := resultOutput.Data.(*v1.JobData_File); ok && resultFileData.File != nil {
				if !expectedPaths[resultFileData.File.Path] {
					// Silently ignore - don't warn, just skip unexpected outputs
					continue
				}
			}
		}
	}

	// Track which outputs we found in result
	foundInResult := make(map[string]bool)

	// Process each expected output from job.Outputs
	// Output handling logic is determined by content presence:
	// - Content filled: output not yet written to disk, materialize writes and verifies hash
	// - Content empty: output already written to disk by runtime, materialize verifies file exists and validates hash
	for _, output := range job.Outputs {
		if output == nil {
			continue
		}

		fileData, ok := output.Data.(*v1.JobData_File)
		if !ok {
			// Silently ignore non-file outputs
			continue
		}

		if fileData.File == nil {
			continue
		}

		outputPath := fileData.File.Path

		// Check if result has this output (prefer result if available, it may have content filled)
		outputToProcess := output
		if jobResult != nil {
			for _, resultOutput := range jobResult.Outputs {
				if resultOutput == nil {
					continue
				}
				resultFileData, ok := resultOutput.Data.(*v1.JobData_File)
				if ok && resultFileData.File != nil && resultFileData.File.Path == outputPath {
					outputToProcess = resultOutput
					foundInResult[outputPath] = true
					break
				}
			}
		}

		processFileData, ok := outputToProcess.Data.(*v1.JobData_File)
		if !ok || processFileData.File == nil {
			continue
		}

		// Determine if content needs to be written or just verified
		if processFileData.File.Content != nil && len(processFileData.File.Content) > 0 {
			// Case 1: Content filled - output file needs to be written (sandboxed execution)
			// Write the file and verify hash

			absPath := outputPath
			if !filepath.IsAbs(absPath) {
				absPath = filepath.Join(dir, outputPath)
			}

			// Create directory if needed
			if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
				return nil, fmt.Errorf("failed to create output directory for %s: %w", outputPath, err)
			}

			// Write content to file
			if err := os.WriteFile(absPath, processFileData.File.Content, os.FileMode(processFileData.File.Mode)); err != nil {
				return nil, fmt.Errorf("failed to write output file %s: %w", outputPath, err)
			}

			// Compute and verify content hash
			computedHash := ComputeContentHash(processFileData.File.Content)
			if processFileData.File.ContentHash != nil && processFileData.File.ContentHash.Value != "" {
				if computedHash != processFileData.File.ContentHash.Value {
					return nil, fmt.Errorf("output file hash mismatch for %s: expected %s, got %s",
						outputPath, processFileData.File.ContentHash.Value, computedHash)
				}
			}

			// Get file info for metadata
			fi, err := os.Stat(absPath)
			if err != nil {
				return nil, fmt.Errorf("failed to stat output file %s: %w", outputPath, err)
			}

			// Create output JobData using helper (file already on disk, return with empty content)
			outputs = append(outputs, createFileJobDataProto(outputPath, fi, computedHash, nil))

		} else {
			// Case 2: Content empty - output file should already exist on disk (local unsandboxed execution)
			// Verify file exists and validate hash

			absPath := outputPath
			if !filepath.IsAbs(absPath) {
				absPath = filepath.Join(dir, outputPath)
			}

			fi, err := os.Stat(absPath)
			if err != nil {
				return nil, fmt.Errorf("output file not found after execution: %s (resolved to %s): %w", outputPath, absPath, err)
			}

			// Verify file hash matches expected
			expectedHash := ""
			if processFileData.File.ContentHash != nil {
				expectedHash = processFileData.File.ContentHash.Value
			}

			hashStr, err := s.verifyFileHashOnDisk(absPath, expectedHash)
			if err != nil {
				return nil, fmt.Errorf("output file hash verification failed for %s: %w", outputPath, err)
			}

			// Create output JobData using helper (file already on disk, return with empty content)
			outputs = append(outputs, createFileJobDataProto(outputPath, fi, hashStr, nil))
		}
	}

	// Validate all expected outputs were found in result (if result was provided and has outputs)
	if jobResult != nil && len(jobResult.Outputs) > 0 {
		for outputPath := range expectedPaths {
			if !foundInResult[outputPath] {
				return nil, fmt.Errorf("job output file not found in result: %s", outputPath)
			}
		}
	}

	return outputs, nil
}

// WriteJobDataToFile writes a JobData file to disk, handling both materialization and verification.
func (s *JobDataStager) WriteJobDataToFile(ctx context.Context, jobData *v1.JobData, outputDir string) error {
	// Validate that job data paths are relative (security check)
	if err := validateJobDataPath(jobData); err != nil {
		return fmt.Errorf("invalid job data: %w", err)
	}

	if jobData.Data == nil {
		return fmt.Errorf("job data has no data content")
	}

	fileData, ok := jobData.Data.(*v1.JobData_File)
	if !ok {
		return fmt.Errorf("job data is not a file type")
	}

	// Determine output path in the output directory (preserving relative structure)
	outputPath := fileData.File.Path
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(outputDir, outputPath)
	}

	// Check if file already exists (e.g., daemon already materialized it)
	fileExists := false
	if _, err := os.Stat(outputPath); err == nil {
		fileExists = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat output file %s: %w", outputPath, err)
	}

	// If file doesn't exist and we have content, write it
	if !fileExists && fileData.File.Content != nil {
		// Create directory if it doesn't exist
		dir := filepath.Dir(outputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory %s: %w", dir, err)
		}

		// Write file content
		if err := os.WriteFile(outputPath, fileData.File.Content, os.FileMode(fileData.File.Mode)); err != nil {
			return fmt.Errorf("failed to write output file %s: %w", outputPath, err)
		}
	} else if !fileExists && fileData.File.Content == nil {
		// File reference mode: file should exist in source location (relative to stager's workDir)
		// Resolve source path relative to workDir
		sourcePath := s.resolveAbsPath(fileData.File.Path)
		sourceInfo, err := os.Stat(sourcePath)
		if err != nil {
			return fmt.Errorf("source file not found at %s: %w", sourcePath, err)
		}

		// Create output directory structure
		dir := filepath.Dir(outputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory %s: %w", dir, err)
		}

		// Copy file from source to output location
		srcContent, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to read source file %s: %w", sourcePath, err)
		}

		if err := os.WriteFile(outputPath, srcContent, os.FileMode(sourceInfo.Mode())); err != nil {
			return fmt.Errorf("failed to copy file to %s: %w", outputPath, err)
		}
	}

	// Verify hash (always, regardless of who wrote it or if it already existed)
	if fileData.File.ContentHash != nil {
		expectedHash := fileData.File.ContentHash.Value
		if expectedHash != "" {
			if _, err := s.verifyFileHashOnDisk(outputPath, expectedHash); err != nil {
				return fmt.Errorf("hash verification failed for output file %s: %w", outputPath, err)
			}
		}
	}

	return nil
}

// WriteJobDataToFileWithSourceDir writes a single JobData file, handling relative paths from a source workDir.
// This is used when input paths are relative to a different working directory than the output directory.
//
// Parameters:
//   - jobData: The JobData to write
//   - outputDir: Destination directory where the file should be written
//   - sourceWorkDir: Original working directory where relative paths should be resolved from
//
// The method preserves the relative path structure when copied to outputDir.
// For example, if sourceWorkDir="/project/build" and the file is "../../src/main.c", it will be
// resolved from "/project/src/main.c" and written to outputDir with the relative structure preserved.
func (s *JobDataStager) WriteJobDataToFileWithSourceDir(ctx context.Context, jobData *v1.JobData, outputDir, sourceWorkDir string) error {
	// Validate that job data paths are relative (security check)
	if err := validateJobDataPath(jobData); err != nil {
		return fmt.Errorf("invalid job data: %w", err)
	}

	if jobData.Data == nil {
		return fmt.Errorf("job data has no data content")
	}

	fileData, ok := jobData.Data.(*v1.JobData_File)
	if !ok {
		return nil
	}

	// Determine output path in the output directory (preserving relative structure)
	if filepath.IsAbs(fileData.File.Path) {
		return fmt.Errorf("output path must be relative, got absolute path: %s", fileData.File.Path)
	}

	// The point of this function is to map file paths so that
	// ouputDir acts as a fakeroot for resolving relative paths in jobData.
	// For example, if sourceDir is /foo/bar/quux, outputDir is /tmp/job123, and jobData has a file with path "../../src/main.c",
	// the original absolute path is  "/foo/src/main.c" and the resulting destination absolute path is "/tmp/job123/foo/bar/src/main.c" and written to that path under outputDir preserving the relative structure.
	fakeroot := outputDir
	fullPath := filepath.Clean(filepath.Join(fakeroot, sourceWorkDir, fileData.File.Path))

	// Check if file already exists
	fileExists := false
	if _, err := os.Stat(fullPath); err == nil {
		fileExists = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat output file %s: %w", fullPath, err)
	}

	if fileExists {
		if err := os.Remove(fullPath); err != nil {
			return fmt.Errorf("failed to remove existing file %s: %w", fullPath, err)
		}
	}

	destinationDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(destinationDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", destinationDir, err)
	}

	if fileData.File.Content != nil {
		// Write file content
		if err := os.WriteFile(fullPath, fileData.File.Content, os.FileMode(fileData.File.Mode)); err != nil {
			return fmt.Errorf("failed to write output file %s: %w", fullPath, err)
		}
	} else if fileData.File.Content == nil {
		// File reference mode: resolve relative to sourceWorkDir
		sourcePath := filepath.Join(sourceWorkDir, fileData.File.Path)
		sourcePath = filepath.Clean(sourcePath) // Normalize to handle ../../ sequences properly

		sourceInfo, err := os.Stat(sourcePath)
		if err != nil {
			return fmt.Errorf("source file not found at %s: %w", sourcePath, err)
		}

		// Copy file from source to output location
		srcContent, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to read source file %s: %w", sourcePath, err)
		}

		if err := os.WriteFile(fullPath, srcContent, os.FileMode(sourceInfo.Mode())); err != nil {
			return fmt.Errorf("failed to copy file to %s: %w", fullPath, err)
		}
	}

	// Verify hash (always, regardless of who wrote it or if it already existed)
	if fileData.File.ContentHash != nil {
		expectedHash := fileData.File.ContentHash.Value
		if expectedHash != "" {
			if _, err := s.verifyFileHashOnDisk(fullPath, expectedHash); err != nil {
				return fmt.Errorf("hash verification failed for output file %s: %w", fullPath, err)
			}
		}
	}

	return nil
}

func (s *JobDataStager) WriteJobDataListToFiles(ctx context.Context, jobDataList []*v1.JobData, outputDir string) error {
	// Validate that all job data paths are relative (security check)
	if err := validateJobDataPathList(jobDataList); err != nil {
		return fmt.Errorf("invalid job data list: %w", err)
	}

	for _, jobData := range jobDataList {
		if err := s.WriteJobDataToFile(ctx, jobData, outputDir); err != nil {
			return err
		}
	}
	return nil
}

// WriteJobDataListToFilesWithSourceDir writes multiple JobData files, handling relative paths from a source directory.
// This is used when materializing inputs that have paths relative to a source workDir (like job.Cwd)
// but need to be copied to a destination workDir (like a tempDir for sandboxed execution).
//
// Parameters:
//   - jobDataList: List of JobData to materialize
//   - outputDir: Destination directory where files should be written
//   - sourceWorkDir: Original working directory where relative paths should be resolved from
func (s *JobDataStager) WriteJobDataListToFilesWithSourceDir(ctx context.Context, jobDataList []*v1.JobData, outputDir, sourceWorkDir string) error {
	// Validate that all job data paths are relative (security check)
	if err := validateJobDataPathList(jobDataList); err != nil {
		return fmt.Errorf("invalid job data list: %w", err)
	}

	for _, jobData := range jobDataList {
		if err := s.WriteJobDataToFileWithSourceDir(ctx, jobData, outputDir, sourceWorkDir); err != nil {
			return err
		}
	}
	return nil
}

// resolveAbsPath converts a path to absolute, relative to workDir if not already absolute
func (s *JobDataStager) resolveAbsPath(filePath string) string {
	if filepath.IsAbs(filePath) {
		return filePath
	}
	return filepath.Join(s.workDir, filePath)
}

// IsWorkdirContained verifies that jobWorkdir is the same as or a child directory of containerWorkdir.
// This is useful for security checks to ensure a job's working directory is within expected boundaries.
// Returns true if jobWorkdir is equal to containerWorkdir or is a descendant of it.
// Returns false if jobWorkdir is outside containerWorkdir or if path resolution fails.
func IsWorkdirContained(containerWorkdir, jobWorkdir string) (bool, error) {
	// Resolve both paths to absolute canonical form
	containerAbs, err := filepath.Abs(filepath.Clean(containerWorkdir))
	if err != nil {
		return false, fmt.Errorf("failed to resolve container workdir %s: %w", containerWorkdir, err)
	}

	jobAbs, err := filepath.Abs(filepath.Clean(jobWorkdir))
	if err != nil {
		return false, fmt.Errorf("failed to resolve job workdir %s: %w", jobWorkdir, err)
	}

	// Check if job workdir is the same as container workdir
	if containerAbs == jobAbs {
		return true, nil
	}

	// Check if job workdir is a child of container workdir
	rel, err := filepath.Rel(containerAbs, jobAbs)
	if err != nil {
		return false, fmt.Errorf("failed to compute relative path: %w", err)
	}

	// If rel starts with "..", job workdir is outside container workdir
	if rel == ".." || (len(rel) > 2 && rel[:3] == ".."+string(filepath.Separator)) {
		return false, nil
	}

	return true, nil
}

// verifyFileHashOnDisk computes the hash of a file on disk and verifies it matches expected hash.
// Returns error if file not found, hash computation fails, or hash doesn't match.
// If expectedHash is empty, only verifies file exists and returns computed hash.
func (s *JobDataStager) verifyFileHashOnDisk(filePath, expectedHash string) (string, error) {
	computedHash, err := ComputeFileHash(filePath)
	if err != nil {
		return "", err
	}

	if expectedHash != "" && computedHash != expectedHash {
		return "", fmt.Errorf("file hash mismatch: expected %s, got %s", expectedHash, computedHash)
	}

	return computedHash, nil
}

// createFileJobDataProto builds a JobData proto for a file with standard metadata.
// Ensures consistent JobData creation across all methods.
func createFileJobDataProto(filePath string, fileInfo os.FileInfo, contentHash string, content []byte) *v1.JobData {
	return &v1.JobData{
		Id:   contentHash,
		Type: v1.JobData_DATA_TYPE_FILE,
		Size: &v1.Size{
			Count: float64(fileInfo.Size()),
			Unit:  v1.SizeUnit_SIZE_UNIT_BYTE,
		},
		Data: &v1.JobData_File{
			File: &v1.FileJobData{
				Path:    filePath,
				Mode:    uint32(fileInfo.Mode()),
				Content: content,
				ContentHash: &v1.Hash{
					Algorithm: v1.HashAlgorithm_HASH_ALGORITHM_SHA256,
					Value:     contentHash,
				},
				IsSymlink: false,
			},
		},
		CreatedAt: &v1.TimeStamp{
			UnixMillis: fileInfo.ModTime().UnixMilli(),
		},
	}
}

// ComputeFileHash computes the SHA256 hash of a file
func ComputeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("failed to compute hash for %s: %w", filePath, err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// ComputeContentHash computes the SHA256 hash of byte content
func ComputeContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash)
}

// VerifyFileJobDataOnDisk verifies that a file job data's on-disk file exists and matches the hash.
// Used for reference data or when file should already be on disk.
// Returns error if file not found or hash doesn't match.
func (s *JobDataStager) VerifyFileJobDataOnDisk(fileData *v1.FileJobData, filePath string) error {
	if fileData == nil {
		return fmt.Errorf("file data is nil")
	}

	// Verify file exists
	_, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file not found: %s: %w", fileData.Path, err)
	}

	// Get expected hash
	expectedHash := ""
	if fileData.ContentHash != nil {
		expectedHash = fileData.ContentHash.Value
	}

	// Verify hash on disk
	_, err = s.verifyFileHashOnDisk(filePath, expectedHash)
	if err != nil {
		return fmt.Errorf("file hash verification failed for %s: %w", fileData.Path, err)
	}

	return nil
}

// VerifyFileJobDataContent verifies that a file job data's content matches its hash.
// Used when file content is present in JobData (sandboxed execution).
// Returns error if content is missing or hash doesn't match.
func (s *JobDataStager) VerifyFileJobDataContent(fileData *v1.FileJobData) error {
	if fileData == nil {
		return fmt.Errorf("file data is nil")
	}

	// Fail if data has no content
	if fileData.Content == nil || len(fileData.Content) == 0 {
		return fmt.Errorf("file data has no content for %s", fileData.Path)
	}

	// Verify content hash matches
	contentHash := ComputeContentHash(fileData.Content)
	if fileData.ContentHash != nil && fileData.ContentHash.Value != "" {
		if contentHash != fileData.ContentHash.Value {
			return fmt.Errorf("content hash mismatch for %s: expected %s, got %s",
				fileData.Path, fileData.ContentHash.Value, contentHash)
		}
	}

	return nil
}

// VerificationMode specifies how to verify output files
type VerificationMode int

const (
	// VerificationModeSaved verifies that outputs are saved to disk with correct hash
	VerificationModeSaved VerificationMode = iota
	// VerificationModeIntegrity verifies integrity only:
	// - For references: verify file exists and hash matches
	// - For content: verify content hash matches (don't verify disk file)
	VerificationModeIntegrity
)

// VerifyJobData verifies a single JobData according to the verification mode.
// Uses a switch statement to support different data types and enable future extensibility.
// For file data:
//   - VerificationModeSaved: File must exist on disk and hash must match
//   - VerificationModeIntegrity: If reference (no content), verify file exists and hash matches.
//     If content is present, verify content hash only.
//
// Returns error if verification fails.
func (s *JobDataStager) VerifyJobData(ctx context.Context, jobData *v1.JobData, workDir string, mode VerificationMode) error {
	if jobData == nil {
		return fmt.Errorf("job data is nil")
	}

	if jobData.Data == nil {
		return fmt.Errorf("job data has no data content")
	}

	// Switch on data type to support different verification strategies
	switch data := jobData.Data.(type) {
	case *v1.JobData_File:
		if data == nil || data.File == nil {
			return fmt.Errorf("job data file is nil")
		}
		return s.verifyFileJobData(ctx, data.File, workDir, mode)

	default:
		// Unknown data types - no verification needed currently
		// This allows for future extensibility with new data types
		return nil
	}
}

// verifyFileJobData verifies a single file JobData according to verification mode
func (s *JobDataStager) verifyFileJobData(ctx context.Context, fileData *v1.FileJobData, workDir string, mode VerificationMode) error {
	filePath := fileData.Path
	if !filepath.IsAbs(filePath) && workDir != "" {
		filePath = filepath.Join(workDir, filePath)
	}

	switch mode {
	case VerificationModeSaved:
		// Verify file exists on disk and hash matches
		if err := s.VerifyFileJobDataOnDisk(fileData, filePath); err != nil {
			return err
		}

		// Verify file size is positive
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("failed to stat file: %s: %w", fileData.Path, err)
		}

		if fileInfo.Size() <= 0 {
			return fmt.Errorf("file size is zero or negative for %s", fileData.Path)
		}

		return nil

	case VerificationModeIntegrity:
		// If content is present, verify content hash only
		if fileData.Content != nil && len(fileData.Content) > 0 {
			return s.VerifyFileJobDataContent(fileData)
		}

		// If reference (no content), verify file exists on disk and hash matches
		return s.VerifyFileJobDataOnDisk(fileData, filePath)

	default:
		return fmt.Errorf("unknown verification mode: %d", mode)
	}
}

// VerifyJobDataList verifies multiple JobData entries according to verification mode.
// Returns error if any verification fails.
func (s *JobDataStager) VerifyJobDataList(ctx context.Context, jobDataList []*v1.JobData, workDir string, mode VerificationMode) error {
	if len(jobDataList) == 0 {
		return nil
	}

	for i, jobData := range jobDataList {
		if err := s.VerifyJobData(ctx, jobData, workDir, mode); err != nil {
			return fmt.Errorf("verification failed for job data [%d]: %w", i, err)
		}
	}
	return nil
}

// VerifyJobResultOutputs verifies that a job result's outputs are valid according to verification mode.
// This is used after job execution to verify the result outputs.
func (s *JobDataStager) VerifyJobResultOutputs(ctx context.Context, jobResult *v1.JobResult, workDir string, mode VerificationMode) error {
	if jobResult == nil {
		return fmt.Errorf("job result is nil")
	}

	if len(jobResult.Outputs) == 0 {
		// No outputs to verify
		return nil
	}

	return s.VerifyJobDataList(ctx, jobResult.Outputs, workDir, mode)
}

// VerifyJobInputs verifies that a job's input files are valid and accessible according to the verification mode.
// This verifies that input file references are valid (files exist and hash matches).
// Typically, inputs are references (already on disk) and use VerificationModeSaved.
func (s *JobDataStager) VerifyJobInputs(ctx context.Context, job *v1.Job, mode VerificationMode) error {
	if job == nil {
		return fmt.Errorf("job is nil")
	}

	if len(job.Inputs) == 0 {
		// No inputs to verify
		return nil
	}

	workDir := job.Cwd
	if workDir == "" {
		workDir = "."
	}

	// Delegate to VerifyJobDataList which handles all the verification logic
	return s.VerifyJobDataList(ctx, job.Inputs, workDir, mode)
}

// EmbedJobData transforms a JobData to ensure all data is embedded (content-based, not reference-based).
// If the JobData is a reference, it loads the referenced file and returns it as FileJobData with embedded content.
// If the JobData is already file/directory data, it ensures content is embedded.
// The returned JobData has all data embedded, suitable for remote execution.
func (s *JobDataStager) EmbedJobData(ctx context.Context, jobData *v1.JobData) (*v1.JobData, error) {
	if jobData == nil {
		return nil, fmt.Errorf("job data cannot be nil")
	}

	// Validate that job data paths are relative (security check)
	if err := validateJobDataPath(jobData); err != nil {
		return nil, fmt.Errorf("invalid job data: %w", err)
	}

	// If it's a reference, load it and convert to file data with content
	if ref := jobData.GetReference(); ref != nil {
		// Load the referenced file
		absPath := s.resolveAbsPath(ref.Id)

		// Read file content
		content, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read referenced file %s: %w", absPath, err)
		}

		// Get file info for permissions
		fileInfo, err := os.Stat(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat file %s: %w", absPath, err)
		}

		// Compute content hash
		hash := ComputeContentHash(content)

		// Create FileJobData with embedded content
		fileData := &v1.FileJobData{
			Path:        ref.Id,
			Mode:        uint32(fileInfo.Mode()),
			Content:     content,
			ContentHash: &v1.Hash{Value: hash},
		}

		// Return as JobData with embedded file
		return &v1.JobData{
			Id:   jobData.Id,
			Type: v1.JobData_DATA_TYPE_FILE,
			Size: &v1.Size{
				Count: float64(len(content)),
				Unit:  v1.SizeUnit_SIZE_UNIT_BYTE,
			},
			Data: &v1.JobData_File{File: fileData},
		}, nil
	}

	// If it's already file data
	if fileData := jobData.GetFile(); fileData != nil {
		// If content is not embedded, load it
		if len(fileData.Content) == 0 {
			absPath := s.resolveAbsPath(fileData.Path)
			content, err := os.ReadFile(absPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", absPath, err)
			}

			// Create new JobData with embedded content
			embeddedFile := &v1.FileJobData{
				Path:          fileData.Path,
				Mode:          fileData.Mode,
				Content:       content,
				ContentHash:   fileData.ContentHash,
				IsSymlink:     fileData.IsSymlink,
				SymlinkTarget: fileData.SymlinkTarget,
			}

			return &v1.JobData{
				Id:   jobData.Id,
				Type: jobData.Type,
				Size: &v1.Size{
					Count: float64(len(content)),
					Unit:  v1.SizeUnit_SIZE_UNIT_BYTE,
				},
				Data: &v1.JobData_File{File: embeddedFile},
			}, nil
		}
		// Content already embedded, return as-is
		return jobData, nil
	}

	// For directory and stream chunk data, return as-is (they already contain what they need)
	// For other types, return unchanged
	return jobData, nil
}

// EmbedJobDataList transforms a list of JobData to ensure all data is embedded.
// Returns a new list where each JobData has all references converted to embedded content.
// The returned list is suitable for remote execution.
func (s *JobDataStager) EmbedJobDataList(ctx context.Context, jobDataList []*v1.JobData) ([]*v1.JobData, error) {
	if len(jobDataList) == 0 {
		return jobDataList, nil
	}

	// Validate that all job data paths are relative (security check)
	if err := validateJobDataPathList(jobDataList); err != nil {
		return nil, fmt.Errorf("invalid job data list: %w", err)
	}

	embeddedList := make([]*v1.JobData, 0, len(jobDataList))

	for _, jobData := range jobDataList {
		if jobData == nil {
			embeddedList = append(embeddedList, nil)
			continue
		}

		embedded, err := s.EmbedJobData(ctx, jobData)
		if err != nil {
			return nil, fmt.Errorf("failed to embed job data %s: %w", jobData.Id, err)
		}

		embeddedList = append(embeddedList, embedded)
	}

	return embeddedList, nil
}
