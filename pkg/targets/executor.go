package targets

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/konveyor/test-harness/pkg/util"
)

// ExecuteCommand runs a command with timeout and captures output
func ExecuteCommand(ctx context.Context, binary string, args []string, workDir string, timeout time.Duration) (*ExecutionResult, error) {
	log := util.GetLogger()
	log.Info("Executing command", "binary", binary, "args", args, "workDir", workDir)

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(execCtx, binary, args...)
	cmd.Dir = workDir

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute
	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Command failed to start or was killed
			return nil, fmt.Errorf("failed to execute command: %w", err)
		}
	}

	result := &ExecutionResult{
		ExitCode: exitCode,
		Duration: duration,
		WorkDir:  workDir,
		Error:    err,
	}

	log.Info("Command completed", "exitCode", exitCode, "duration", duration)

	if exitCode != 0 {
		return nil, fmt.Errorf("command failed with exit code: %d", exitCode)
	}

	return result, nil
}

// PrepareWorkDir creates a unique work directory for test execution
func PrepareWorkDir(baseDir, testName string) (string, error) {
	// Sanitize test name to avoid issues with special characters and spaces
	sanitized := sanitizeName(testName)
	timestamp := time.Now().Format("20060102-150405")
	workDir := filepath.Join(baseDir, fmt.Sprintf("%s-%s", sanitized, timestamp))

	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create work directory: %w", err)
	}

	return workDir, nil
}

// sanitizeName removes or replaces characters that might cause issues in file paths
func sanitizeName(name string) string {
	// Replace spaces and special characters with hyphens
	result := ""
	for _, ch := range name {
		if ch == ' ' || ch == '/' || ch == '\\' {
			result += "-"
		} else if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			result += string(ch)
		}
	}
	return result
}

// LogResult logs the execution result details
func LogResult(log logr.Logger, result *ExecutionResult) {
	log.Info("Execution result",
		"exitCode", result.ExitCode,
		"duration", result.Duration,
		"outputFile", result.OutputFile,
	)

	if result.Stdout != "" {
		log.V(1).Info("Stdout", "output", result.Stdout)
	}

	if result.Stderr != "" {
		log.V(1).Info("Stderr", "output", result.Stderr)
	}
}
