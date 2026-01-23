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
	log.V(2).Info("Test config", "config", test.Analysis)

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
	inputPath, err := k.prepareInput(ctx, &test.Analysis, testDir)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare input: %w", err)
	}

	// Handle rules that may be Git URLs
	preparedRules, err := k.prepareRules(ctx, &test.Analysis, workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare rules: %w", err)
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

	// Build kantra command arguments with prepared rules
	args := k.buildArgs(test.Analysis, inputPath, absOutputDir, k.mavenSettings, preparedRules)

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

// buildArgsWithPreparedRules constructs the kantra analyze command arguments with prepared rules
func (k *KantraTarget) buildArgs(analysis config.AnalysisConfig, inputPath, outputDir, mavenSettings string, preparedRules []string) []string {
	args := []string{"analyze", "--context-lines", strconv.Itoa(analysis.ContextLines)}

	// Input application (now using the prepared input path)
	args = append(args, "--input", inputPath)

	// Output directory (now passed as parameter, already absolute)
	args = append(args, "--output", outputDir)

	args = append(args, "--skip-static-report")

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
	// Use prepared rules that have been cloned/resolved
	if len(preparedRules) > 0 {
		for _, rule := range preparedRules {
			args = append(args, "--rules", rule)
		}
	}

	if analysis.DisableDefaultRules {
		fmt.Printf("disableDefaultRules")
		args = append(args, "--enable-default-rulesets=false")
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

// prepareInput handles git URLs, local paths, and binary files
// Returns the local path to use as input for kantra
func (k *KantraTarget) prepareInput(ctx context.Context, analysis *config.AnalysisConfig, workDir string) (string, error) {
	log := util.GetLogger()
	application := analysis.Application

	// Check if it's a binary file (.jar, .war, .ear)
	if IsBinaryFile(analysis.Application) {
		log.Info("Detected binary input", "file", analysis.Application)
		return k.prepareBinary(analysis.Application, workDir)
	}

	// Check if we have parsed Git components
	if analysis.ApplicationGitComponents != nil {
		// Clone the repository using parsed components
		return CloneGitRepository(ctx, analysis.ApplicationGitComponents, workDir, "source")
	}

	// It's a local path or binary reference
	// Handle binary: prefix (legacy support)
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

// prepareRules handles rules that may be Git URLs or local paths
// Returns a list of prepared rule paths
func (k *KantraTarget) prepareRules(ctx context.Context, analysis *config.AnalysisConfig, workDir string) ([]string, error) {
	if len(analysis.Rules) == 0 {
		return nil, nil
	}

	log := util.GetLogger()
	preparedRules := make([]string, 0, len(analysis.Rules))

	for i, rule := range analysis.Rules {
		// Check if we have parsed Git components for this rule
		if i < len(analysis.RulesGitComponents) && analysis.RulesGitComponents[i] != nil {
			log.Info("Detected Git URL for rule", "rule", rule)
			// Clone the repository to a unique directory for this rule
			cloneName := fmt.Sprintf("rules-%d", i)
			clonedPath, err := CloneGitRepository(ctx, analysis.RulesGitComponents[i], workDir, cloneName)
			if err != nil {
				return nil, fmt.Errorf("failed to clone rules repository %s: %w", rule, err)
			}
			preparedRules = append(preparedRules, clonedPath)
		} else {
			// Local path - use as-is
			preparedRules = append(preparedRules, rule)
		}
	}

	return preparedRules, nil
}

// prepareBinary validates and resolves the path to a binary file (.jar, .war, .ear)
// Returns the absolute path to the binary file
func (k *KantraTarget) prepareBinary(binaryPath, testDir string) (string, error) {
	log := util.GetLogger()

	// Check if path is absolute
	if filepath.IsAbs(binaryPath) {
		if _, err := os.Stat(binaryPath); err != nil {
			return "", fmt.Errorf("binary file not found: %w", err)
		}
		log.Info("Using absolute binary path", "path", binaryPath)
		return binaryPath, nil
	}

	// Relative path - resolve relative to test directory
	absPath := filepath.Join(testDir, binaryPath)

	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("binary file not found at %s: %w", absPath, err)
	}

	log.Info("Resolved relative binary path", "original", binaryPath, "resolved", absPath)
	return absPath, nil
}
