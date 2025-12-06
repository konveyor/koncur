package targets

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/konveyor/analyzer-lsp/provider"
	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/util"
)

// KantraTarget implements Target for Kantra
type KantraTarget struct {
	binaryPath    string
	mavenSettings string
}

// NewKantraTarget creates a new Kantra target
func NewKantraTarget(cfg *config.KantraConfig) (*KantraTarget, error) {
	var binaryPath string
	var mavenSettings string

	// Use configured path if provided
	if cfg != nil && cfg.BinaryPath != "" {
		binaryPath = cfg.BinaryPath
	} else {
		// Find kantra binary in PATH
		var err error
		binaryPath, err = exec.LookPath("kantra")
		if err != nil {
			return nil, fmt.Errorf("kantra binary not found in PATH: %w", err)
		}
	}

	// Get maven settings from config
	if cfg != nil {
		mavenSettings = cfg.MavenSettings
	}

	return &KantraTarget{
		binaryPath:    binaryPath,
		mavenSettings: mavenSettings,
	}, nil
}

// Name returns the target name
func (k *KantraTarget) Name() string {
	return "kantra"
}

// Execute runs kantra analyze
func (k *KantraTarget) Execute(ctx context.Context, test *config.TestDefinition) (*ExecutionResult, error) {
	log := util.GetLogger()
	log.Info("Executing Kantra analysis", "test", test.Name)

	// Validate maven settings requirement
	if test.RequireMavenSettings && k.mavenSettings == "" {
		return nil, fmt.Errorf("test requires maven settings but none configured in target config")
	}

	// Get test directory (where test.yaml is located)
	testDir := test.GetTestDir()
	if testDir == "" {
		return nil, fmt.Errorf("test directory not available")
	}

	// Prepare work directory for execution logs/metadata
	workDir, err := PrepareWorkDir(test.GetWorkDir(), test.Name)
	if err != nil {
		return nil, err
	}

	// Handle application input (clone git repo to test-dir/source if needed)
	inputPath, err := k.prepareInput(ctx, test.Analysis.Application, test.Name, testDir)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare input: %w", err)
	}

	// Create output directory with absolute path
	outputDir := filepath.Join(workDir, "output")
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute output path: %w", err)
	}
	if err := os.MkdirAll(absOutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build kantra command arguments
	args := k.buildArgs(test.Analysis, inputPath, absOutputDir, k.mavenSettings)

	// Execute kantra
	result, err := ExecuteCommand(ctx, k.binaryPath, args, workDir, test.GetTimeout())
	if err != nil {
		return nil, err
	}

	// Set the output file path (absOutputDir is already absolute)
	result.OutputFile = filepath.Join(absOutputDir, "output.yaml")

	LogResult(log, result)

	return result, nil
}

// buildArgs constructs the kantra analyze command arguments
func (k *KantraTarget) buildArgs(analysis config.AnalysisConfig, inputPath, outputDir, mavenSettings string) []string {
	args := []string{"analyze", "--context-lines", strconv.Itoa(analysis.ContextLines)}

	// Input application (now using the prepared input path)
	args = append(args, "--input", inputPath)

	// Output directory (now passed as parameter, already absolute)
	args = append(args, "--output", outputDir)

	// Label selector (if specified)
	if analysis.LabelSelector != "" {
		args = append(args, "--label-selector", analysis.LabelSelector)
	}

	if analysis.IncidentSelector != "" {
		args = append(args, "--incident-selector", analysis.IncidentSelector)
	}

	// Maven settings (from test-level configuration)
	if mavenSettings != "" {
		args = append(args, "--maven-settings", mavenSettings)
	}

	if len(analysis.Target) > 0 {
		for _, target := range analysis.Target {
			args = append(args, "-t", target)
		}
	}
	if len(analysis.Source) > 0 {
		for _, source := range analysis.Source {
			args = append(args, "-s", source)
		}
	}
	if len(analysis.Rules) > 0 {
		for _, rule := range analysis.Rules {
			args = append(args, "--rules", rule)
		}
	}

	// Analysis mode
	switch analysis.AnalysisMode {
	case provider.SourceOnlyAnalysisMode:
		args = append(args, "--mode", "source-only")
	case provider.FullAnalysisMode:
		// Full is the default, but we can be explicit
		args = append(args, "--mode", "full")
	}

	// Use container mode instead of run-local to avoid dependency issues
	args = append(args, "--run-local=false")

	// Allow overwriting existing output
	args = append(args, "--overwrite")

	return args
}

// prepareInput handles git URLs and local paths
// Returns the local path to use as input for kantra
func (k *KantraTarget) prepareInput(ctx context.Context, application, testName, workDir string) (string, error) {
	log := util.GetLogger()

	// Check if it's a git URL (starts with http://, https://, or git@)
	// or contains a git reference (has #branch)
	isGitURL := strings.HasPrefix(application, "http://") ||
		strings.HasPrefix(application, "https://") ||
		strings.HasPrefix(application, "git@")

	if !isGitURL {
		// It's a local path or binary reference
		// Handle binary: prefix
		if strings.HasPrefix(application, "binary:") {
			// Extract the binary file name
			binaryFile := application[7:] // Remove "binary:" prefix
			// For now, just return the binary file as-is
			// In the future, we might need to look for it in a specific directory
			return binaryFile, nil
		}
		// Return as-is for local paths
		return application, nil
	}

	// Parse git URL and reference
	var gitURL, gitRef string
	if strings.Contains(application, "#") {
		parts := strings.SplitN(application, "#", 2)
		gitURL = parts[0]
		if len(parts) > 1 {
			gitRef = parts[1]
		}
	} else {
		gitURL = application
	}

	// Clone the git repository into workDir/source folder
	inputDir := filepath.Join(workDir, "source")

	// Get absolute path
	absInputDir, err := filepath.Abs(inputDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if directory already exists
	if _, err := os.Stat(absInputDir); err == nil {
		log.Info("Repository already exists, skipping clone", "dest", absInputDir)
		return absInputDir, nil
	}

	log.Info("Cloning git repository", "url", gitURL, "ref", gitRef, "dest", absInputDir)

	// Build git clone command
	var gitArgs []string
	if gitRef != "" {
		gitArgs = []string{"clone", "--depth", "1", "--branch", gitRef, gitURL, absInputDir}
	} else {
		gitArgs = []string{"clone", "--depth", "1", gitURL, absInputDir}
	}

	// Execute git clone
	result, err := ExecuteCommand(ctx, "git", gitArgs, ".", 5*60*1000000000) // 5 minute timeout for clone
	if err != nil {
		log.Info("Git clone failed", "error", err.Error(), "exitCode", result.ExitCode, "stderr", result.Stderr)
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	log.Info("Git clone completed successfully")
	return absInputDir, nil
}
