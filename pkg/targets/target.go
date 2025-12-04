package targets

import (
	"context"
	"time"

	"github.com/konveyor/test-harness/pkg/config"
)

// Target represents a tool that can be executed (kantra, tackle, kai)
type Target interface {
	// Name returns the target name
	Name() string

	// Execute runs the analysis and returns the result
	Execute(ctx context.Context, test *config.TestDefinition) (*ExecutionResult, error)
}

// ExecutionResult contains the results of executing a target
type ExecutionResult struct {
	// ExitCode from the process
	ExitCode int

	// Duration of execution
	Duration time.Duration

	// OutputFile path to the generated output.yaml
	OutputFile string

	// WorkDir where the execution happened
	WorkDir string

	// Stdout captured from execution
	Stdout string

	// Stderr captured from execution
	Stderr string

	// Error if execution failed
	Error error
}
