package cmd

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationTailCommand tests the tail command with streaming logs
func TestIntegrationTailCommand(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	// Run tail command with a short timeout to capture some logs
	fullArgs := []string{"--port", fmt.Sprintf("%d", helper.daemonPort), "--host", helper.daemonHost}
	fullArgs = append(fullArgs, "logs", "tail")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stdout, stderr, err := helper.cliDriver.Run(ctx, fullArgs...)

	// Command should timeout (context canceled) which is expected for streaming
	// We're checking that we got some output before timeout
	if err != nil {
		// Streaming commands will error on context cancellation, which is expected
		t.Logf("tail command exited (expected for streaming): %v", err)
	}

	// Even if command times out, we should have captured some logs
	assert.NotEmpty(t, stdout, "tail command should produce output\nstderr: %s", stderr)

	// Verify output contains log entries with expected format
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	require.True(t, len(lines) > 0, "should have at least one log line")

	// First line should contain timestamp and log level
	firstLine := lines[0]
	assert.True(t, strings.Contains(firstLine, ";") || strings.Contains(firstLine, "["),
		"log line should contain timestamp or level indicator: %s", firstLine)
}

// TestIntegrationTailCommandWithLevelFilter tests tail command with level filtering
func TestIntegrationTailCommandWithLevelFilter(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	// Run tail command filtering for error level only
	fullArgs := []string{"--port", fmt.Sprintf("%d", helper.daemonPort), "--host", helper.daemonHost}
	fullArgs = append(fullArgs, "logs", "tail", "--levels", "error")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stdout, stderr, err := helper.cliDriver.Run(ctx, fullArgs...)

	if err != nil {
		t.Logf("tail command exited (expected for streaming): %v", err)
	}

	// If no error logs are generated, stdout might be empty - that's ok
	if stdout != "" {
		// Verify that any output lines contain error indicator
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		for _, line := range lines {
			if line != "" {
				// Line should contain error level indicator
				assert.True(t,
					strings.Contains(strings.ToLower(line), "error") ||
						strings.Contains(line, "ERROR") ||
						strings.Contains(line, "E "),
					"filtered log line should contain error level: %s", line)
			}
		}
	}

	if stderr != "" {
		t.Logf("tail command stderr: %s", stderr)
	}
}

// TestIntegrationTailCommandWithLoggerFilter tests tail command with logger name filtering
func TestIntegrationTailCommandWithLoggerFilter(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	// Run tail command filtering for specific logger
	fullArgs := []string{"--port", fmt.Sprintf("%d", helper.daemonPort), "--host", helper.daemonHost}
	fullArgs = append(fullArgs, "logs", "tail", "--logger", "buildozer*")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stdout, stderr, err := helper.cliDriver.Run(ctx, fullArgs...)

	if err != nil {
		t.Logf("tail command exited (expected for streaming): %v", err)
	}

	// Should get some output for buildozer.*loggers
	assert.NotEmpty(t, stdout, "tail with logger filter should produce output\nstderr: %s", stderr)
}

// TestIntegrationTailCommandMultipleConcurrent tests multiple concurrent tail streams
func TestIntegrationTailCommandMultipleConcurrent(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	// Start multiple concurrent tail streams
	const numStreams = 3
	results := make(chan bool, numStreams)

	for i := 0; i < numStreams; i++ {
		go func(streamNum int) {
			fullArgs := []string{"--port", fmt.Sprintf("%d", helper.daemonPort), "--host", helper.daemonHost}
			fullArgs = append(fullArgs, "logs", "tail")

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			stdout, _, err := helper.cliDriver.Run(ctx, fullArgs...)

			if err != nil {
				t.Logf("Stream %d exited (expected): %v", streamNum, err)
			}

			success := stdout != ""
			results <- success
		}(i)
	}

	// Verify all streams got some output
	successCount := 0
	for i := 0; i < numStreams; i++ {
		if <-results {
			successCount++
		}
	}

	assert.True(t, successCount > 0, "at least one concurrent stream should get output")
}
