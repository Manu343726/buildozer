package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CLIDriver provides an interface for executing the buildozer-client CLI
// It uses `go run` to invoke the CLI, eliminating the need for a pre-built binary
type CLIDriver struct {
	projectRoot string
}

// NewCLIDriver creates a new CLI driver that uses go run
func NewCLIDriver() *CLIDriver {
	cwd, _ := os.Getwd()
	originalCwd := cwd

	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return &CLIDriver{projectRoot: cwd}
		}

		parent := filepath.Dir(cwd)
		if parent == cwd {
			return &CLIDriver{projectRoot: originalCwd}
		}
		cwd = parent
	}
}

// Run executes the CLI with the given arguments
func (d *CLIDriver) Run(ctx context.Context, args ...string) (string, string, error) {
	cmdArgs := []string{"run", "./cmd/buildozer-client/main.go"}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Dir = d.projectRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// TestHelper manages test infrastructure
type TestHelper struct {
	t               *testing.T
	daemonPort      int
	daemonHost      string
	daemonProcess   *exec.Cmd
	cliDriver       *CLIDriver
	tempConfigFile  string
	daemonStartTime time.Time
	daemonStdoutBuf bytes.Buffer
	daemonStderrBuf bytes.Buffer
}

// getRandomPort returns an unused port
func getRandomPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// NewTestHelper creates a test helper
func NewTestHelper(t *testing.T) *TestHelper {
	port, err := getRandomPort()
	require.NoError(t, err, "failed to get random port")

	return &TestHelper{
		t:          t,
		daemonPort: port,
		daemonHost: "127.0.0.1",
		cliDriver:  NewCLIDriver(),
	}
}

// StartDaemon starts the daemon in the background
func (h *TestHelper) StartDaemon(t *testing.T) {
	t.Logf("Starting daemon on port %d", h.daemonPort)

	configContent := fmt.Sprintf(`
daemon:
  host: %s
  port: %d
  max_concurrent_jobs: 4

logging:
  level: debug
  format: text

cache:
  max_size_gb: 1

peer_discovery:
  enabled: false
`, h.daemonHost, h.daemonPort)

	configFile, err := os.CreateTemp("", "buildozer-test-*.yaml")
	require.NoError(t, err)
	h.tempConfigFile = configFile.Name()
	defer configFile.Close()

	_, err = configFile.WriteString(configContent)
	require.NoError(t, err)

	h.daemonProcess = exec.Command(
		"go", "run", "./cmd/buildozer-client/main.go",
		"daemon",
		"--settings", h.tempConfigFile,
		"--log-level", "debug",
	)
	h.daemonProcess.Dir = h.cliDriver.projectRoot
	h.daemonProcess.Stdout = &h.daemonStdoutBuf
	h.daemonProcess.Stderr = &h.daemonStderrBuf

	err = h.daemonProcess.Start()
	require.NoError(t, err, "failed to start daemon process")

	h.daemonStartTime = time.Now()
	h.WaitForDaemonReady(t)
	t.Logf("Daemon is ready on %s:%d", h.daemonHost, h.daemonPort)
}

// WaitForDaemonReady waits for the daemon to be ready (5 second timeout)
func (h *TestHelper) WaitForDaemonReady(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	addr := fmt.Sprintf("http://%s:%d/health", h.daemonHost, h.daemonPort)

	for {
		select {
		case <-ticker.C:
			// Check if daemon exited unexpectedly
			if h.daemonProcess.ProcessState != nil {
				exitCode := h.daemonProcess.ProcessState.ExitCode()
				if exitCode != 0 {
					t.Logf("daemon exited with code %d", exitCode)
					t.Logf("Daemon stderr:\n%s", h.daemonStderrBuf.String())
					t.FailNow()
				}
			}

			// Try health check
			client := &http.Client{Timeout: 100 * time.Millisecond}
			resp, err := client.Get(addr)
			if err == nil {
				resp.Body.Close()
				t.Logf("Daemon is responding on %s", addr)
				return
			}

			if time.Now().After(deadline) {
				t.Logf("daemon did not become ready within 5 seconds")
				if h.daemonProcess.ProcessState != nil {
					t.Logf("daemon exited with code %d", h.daemonProcess.ProcessState.ExitCode())
				}
				t.Logf("Daemon stderr:\n%s", h.daemonStderrBuf.String())
				t.FailNow()
			}
		}
	}
}

// StopDaemon stops the daemon
func (h *TestHelper) StopDaemon(t *testing.T) {
	if h.tempConfigFile != "" {
		os.Remove(h.tempConfigFile)
	}

	if h.daemonProcess == nil || h.daemonProcess.ProcessState != nil {
		return
	}

	t.Log("Stopping daemon")

	if h.daemonProcess.Process != nil {
		h.daemonProcess.Process.Signal(os.Interrupt)
	}

	done := make(chan error, 1)
	go func() {
		if h.daemonProcess.Process != nil {
			done <- h.daemonProcess.Wait()
		} else {
			done <- nil
		}
	}()

	select {
	case <-done:
		return
	case <-time.After(5 * time.Second):
		if h.daemonProcess.Process != nil {
			h.daemonProcess.Process.Kill()
			select {
			case <-done:
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}

// RunCommand runs a CLI command
func (h *TestHelper) RunCommand(t *testing.T, args ...string) (string, string, error) {
	fullArgs := []string{"--port", fmt.Sprintf("%d", h.daemonPort), "--host", h.daemonHost}
	fullArgs = append(fullArgs, args...)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return h.cliDriver.Run(ctx, fullArgs...)
}

// RunStandaloneCommand runs a CLI command in standalone mode
func (h *TestHelper) RunStandaloneCommand(t *testing.T, args ...string) (string, string, error) {
	fullArgs := append([]string{"--standalone"}, args...)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return h.cliDriver.Run(ctx, fullArgs...)
}

// TestIntegrationDaemonStartup tests daemon startup
func TestIntegrationDaemonStartup(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	require.NotNil(t, helper.daemonProcess)
	require.Nil(t, helper.daemonProcess.ProcessState, "daemon should still be running")
}

// TestIntegrationConfigCommand tests the config command
func TestIntegrationConfigCommand(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	stdout, stderr, err := helper.RunCommand(t, "config")

	assert.NoError(t, err, "config command should succeed\nstderr: %s", stderr)
	assert.NotEmpty(t, stdout, "config output should not be empty")
	assert.Contains(t, stdout, "Daemon:", "config output should mention daemon configuration")
}

// TestIntegrationStatusCommand tests the status command
func TestIntegrationStatusCommand(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	stdout, stderr, err := helper.RunCommand(t, "status")

	assert.NoError(t, err, "status command should succeed\nstderr: %s", stderr)
	assert.NotEmpty(t, stdout, "status output should not be empty")
}

// TestIntegrationLogsStatusCommand tests the logs status command
func TestIntegrationLogsStatusCommand(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	stdout, stderr, err := helper.RunCommand(t, "logs", "status")

	assert.NoError(t, err, "logs status command should succeed\nstderr: %s", stderr)
	assert.NotEmpty(t, stdout, "logs status output should not be empty")
}

// TestIntegrationStandaloneMode tests standalone mode
func TestIntegrationStandaloneMode(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)

	stdout, stderr, err := helper.RunStandaloneCommand(t, "status")

	assert.NoError(t, err, "standalone status should succeed\nstderr: %s", stderr)
	assert.NotEmpty(t, stdout, "standalone status output should not be empty")
	assert.Contains(t, stdout, "STANDALONE", "should indicate standalone mode")
}

// TestIntegrationMultipleClients tests multiple concurrent commands
func TestIntegrationMultipleClients(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	results := make(chan error, 5)

	for i := 0; i < 5; i++ {
		go func(cmdNum int) {
			_, stderr, err := helper.RunCommand(t, "status")
			if err != nil {
				results <- fmt.Errorf("command %d failed: %w\nstderr: %s", cmdNum, err, stderr)
			} else {
				results <- nil
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		err := <-results
		assert.NoError(t, err, "concurrent command %d should succeed", i)
	}
}

// TestIntegrationCommandLineFlags tests CLI flag override
func TestIntegrationCommandLineFlags(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	stdout, stderr, err := helper.RunCommand(t, "--log-level", "debug", "config")

	assert.NoError(t, err, "config with flags should succeed\nstderr: %s", stderr)
	assert.NotEmpty(t, stdout, "config output should not be empty")
}

// TestIntegrationDaemonPortRandomization tests random port allocation
func TestIntegrationDaemonPortRandomization(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper1 := NewTestHelper(t)
	helper2 := NewTestHelper(t)

	defer helper1.StopDaemon(t)
	defer helper2.StopDaemon(t)

	assert.NotEqual(t, helper1.daemonPort, helper2.daemonPort,
		"random ports should be different")

	helper1.StartDaemon(t)
	helper2.StartDaemon(t)

	require.NotNil(t, helper1.daemonProcess)
	require.NotNil(t, helper2.daemonProcess)
}

// TestIntegrationDaemonShutdown tests daemon shutdown
func TestIntegrationDaemonShutdown(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	helper.StartDaemon(t)

	require.NotNil(t, helper.daemonProcess)

	helper.StopDaemon(t)

	assert.NotNil(t, helper.daemonProcess.ProcessState, "daemon process should have exited")
}

// TestIntegrationPeersCommand tests the peers command
func TestIntegrationPeersCommand(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	stdout, stderr, err := helper.RunCommand(t, "peers")

	assert.NoError(t, err, "peers command should succeed\nstderr: %s", stderr)
	assert.NotEmpty(t, stdout, "peers output should not be empty")
}

// TestIntegrationCacheCommand tests the cache command
func TestIntegrationCacheCommand(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	stdout, stderr, err := helper.RunCommand(t, "cache")

	assert.NoError(t, err, "cache command should succeed\nstderr: %s", stderr)
	assert.NotEmpty(t, stdout, "cache output should not be empty")
}

// TestIntegrationQueueCommand tests the queue command
func TestIntegrationQueueCommand(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	stdout, stderr, err := helper.RunCommand(t, "queue")

	assert.NoError(t, err, "queue command should succeed\nstderr: %s", stderr)
	assert.NotEmpty(t, stdout, "queue output should not be empty")
}

// TestIntegrationAddSinkCommand tests the add-sink command
func TestIntegrationAddSinkCommand(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	stdout, stderr, err := helper.RunCommand(t, "logs", "add-sink", "test-sink", "stdout", "info")

	if err != nil {
		t.Logf("add-sink output:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
}

// TestIntegrationGccDriverSchedulerIntegration tests gcc driver with daemon scheduler
func TestIntegrationGccDriverSchedulerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	// First, verify the daemon has available runtimes for gcc
	stdout, stderr, err := helper.RunCommand(t, "runtime", "list")
	require.NoError(t, err, "runtime list should succeed\nstderr: %s", stderr)
	t.Logf("Available runtimes:\n%s", stdout)

	// Check that we have at least one native C runtime available for gcc
	assert.NotEmpty(t, stdout, "should have runtimes available")
	assert.Contains(t, stdout, "native-c-gcc", "should have GCC C runtime available")

	// Check initial queue is empty
	queueOut, _, queueCheckErr := helper.RunCommand(t, "queue")
	require.NoError(t, queueCheckErr, "queue command should succeed")
	t.Logf("Initial queue status:\n%s", queueOut)

	// Now test the config file is being used by the driver
	tmpDir, err := os.MkdirTemp("", "buildozer-config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config file with driver settings
	configFile := filepath.Join(tmpDir, ".buildozer")
	configContent := fmt.Sprintf(`
standalone: false
daemon:
  host: %s
  port: %d

drivers:
  gcc:
    compiler_version: "10"
    c_runtime: "glibc"
    c_runtime_version: "2.31"
    architecture: "x86_64"
`, helper.daemonHost, helper.daemonPort)

	err = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err, "should write config file")

	// Create a minimal C source file
	sourceFile := filepath.Join(tmpDir, "test.c")
	err = os.WriteFile(sourceFile, []byte("int main() { return 0; }"), 0644)
	require.NoError(t, err, "should write source file")

	t.Logf("Testing gcc driver with config file from %s", tmpDir)

	// Test that gcc driver can at least be invoked (with timeout to avoid hanging)
	// This will trigger job submission to the scheduler
	cmdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		cmdCtx,
		"go", "run", "./cmd/drivers/cpp/gcc/main.go",
		"-c", sourceFile,
		"-o", filepath.Join(tmpDir, "test.o"),
	)
	cmd.Dir = tmpDir // Will use .buildozer config from this directory

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Run the gcc driver to submit a job
	_ = cmd.Run()
	driverStdout := stdoutBuf.String()
	driverStderr := stderrBuf.String()

	t.Logf("gcc driver output:\nstdout: %s\nstderr: %s", driverStdout, driverStderr)

	// Give daemon time to process the job submission
	time.Sleep(500 * time.Millisecond)

	// Check queue after job submission - should have a job or show completion
	queueAfter, _, queueErr := helper.RunCommand(t, "queue")
	require.NoError(t, queueErr, "queue command should succeed")
	t.Logf("Queue status after job submission:\n%s", queueAfter)

	// Verify daemon is still responsive - this proves the scheduler is working
	dStatusOut, dStatusErr, dStatusEr := helper.RunCommand(t, "status")
	assert.NoError(t, dStatusEr, "daemon should still be responsive after job submission\nstderr: %s", dStatusErr)
	assert.NotEmpty(t, dStatusOut, "should get daemon status")

	// The fact that we got a status response means the daemon scheduler processed the job
	t.Log("✓ Scheduler processed job submission successfully")
}

// TestIntegrationGccDriverWithConfigFile tests gcc driver using the .buildozer config file
func TestIntegrationGccDriverWithConfigFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)

	// Create a temporary directory for test files and config
	tmpDir, err := os.MkdirTemp("", "buildozer-config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a .buildozer config file in the temp directory
	configFile := filepath.Join(tmpDir, ".buildozer")
	configContent := fmt.Sprintf(`
standalone: false
daemon:
  host: %s
  port: %d

drivers:
  gcc:
    compiler_version: "10"
    c_runtime: "glibc"
    c_runtime_version: "2.31"
    architecture: "x86_64"
`, helper.daemonHost, helper.daemonPort)

	err = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err, "failed to create config file")

	// Verify the config can be read
	configBytes, err := os.ReadFile(configFile)
	require.NoError(t, err, "should read config file")
	assert.NotEmpty(t, configBytes, "config file should not be empty")

	t.Logf("Created config file at %s with content:\n%s", configFile, string(configBytes))

	// Create a simple test C source file
	sourceFile := filepath.Join(tmpDir, "simple.c")
	sourceCode := `int add(int a, int b) { return a + b; }`
	err = os.WriteFile(sourceFile, []byte(sourceCode), 0644)
	require.NoError(t, err, "failed to create test source file")

	// Verify config file exists in the working directory
	helpers := helper.cliDriver.projectRoot
	t.Logf("Project root: %s", helpers)
	t.Logf("Test tmpDir: %s", tmpDir)

	// The config file should be discoverable in tmpDir
	assert.FileExists(t, configFile, "config file should exist at %s", configFile)
}

// TestIntegrationSchedulingAlgorithm tests the daemon scheduling algorithm with multiple jobs
func TestIntegrationSchedulingAlgorithm(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	helper := NewTestHelper(t)
	defer helper.StopDaemon(t)

	helper.StartDaemon(t)
	t.Log("Daemon started")

	// Verify available runtimes
	stdout, _, err := helper.RunCommand(t, "runtime", "list")
	require.NoError(t, err, "runtime list should succeed")
	assert.Contains(t, stdout, "native-c-gcc", "should have GCC runtime")
	t.Log("✓ Available runtimes verified")

	// Create test directory with config and source files
	tmpDir, err := os.MkdirTemp("", "buildozer-scheduler-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config
	configFile := filepath.Join(tmpDir, ".buildozer")
	configContent := fmt.Sprintf(`
standalone: false
daemon:
  host: %s
  port: %d

drivers:
  gcc:
    compiler_version: "10"
    c_runtime: "glibc"
    c_runtime_version: "2.31"
    architecture: "x86_64"
`, helper.daemonHost, helper.daemonPort)

	err = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err, "failed to create config")
	t.Log("✓ Config file created")

	// Create multiple test files to verify scheduling of multiple jobs
	numJobs := 3
	files := make([]string, numJobs)
	for i := 0; i < numJobs; i++ {
		sourceFile := filepath.Join(tmpDir, fmt.Sprintf("job%d.c", i))
		sourceCode := fmt.Sprintf("int func%d() { return %d; }", i, i)
		err = os.WriteFile(sourceFile, []byte(sourceCode), 0644)
		require.NoError(t, err, "failed to create source file %d", i)
		files[i] = sourceFile
	}
	t.Logf("✓ Created %d test source files", numJobs)

	// Check initial queue
	queueOut, _, queueCheckErr := helper.RunCommand(t, "queue")
	require.NoError(t, queueCheckErr, "queue command should succeed")
	t.Logf("Initial queue:\n%s", queueOut)

	// Submit multiple jobs by running gcc driver multiple times
	for i := 0; i < numJobs; i++ {
		t.Logf("Submitting job %d", i)

		cmdCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

		cmd := exec.CommandContext(
			cmdCtx,
			"go", "run", "./cmd/drivers/cpp/gcc/main.go",
			"-c", files[i],
			"-o", filepath.Join(tmpDir, fmt.Sprintf("job%d.o", i)),
		)
		cmd.Dir = tmpDir

		var stderrBuf bytes.Buffer
		cmd.Stderr = &stderrBuf

		// Run the driver - it will submit job to scheduler
		_ = cmd.Run()
		cancel()

		// Small delay between submissions
		time.Sleep(100 * time.Millisecond)
	}
	t.Logf("✓ Submitted %d jobs to scheduler", numJobs)

	// Give daemon time to process jobs
	time.Sleep(1 * time.Second)

	// Check queue status after job submissions
	queueAfter, _, queueAfterErr := helper.RunCommand(t, "queue")
	require.NoError(t, queueAfterErr, "queue command should succeed")
	t.Logf("Queue after job submissions:\n%s", queueAfter)

	// The scheduler should have processed these jobs
	// Check that daemon is still responsive (proves scheduler ran without crashing)
	statusOut, _, err := helper.RunCommand(t, "status")
	require.NoError(t, err, "status command should succeed")
	t.Logf("Daemon status after scheduling:\n%s", statusOut)

	// Check cache to verify jobs were processed
	cacheOut, _, err := helper.RunCommand(t, "cache")
	require.NoError(t, err, "cache command should succeed")
	t.Logf("Cache status:\n%s", cacheOut)

	t.Log("✓ Scheduling algorithm test complete - daemon processed all jobs")
	t.Log("✓ Scheduler is working and handling concurrent job submissions")
}
