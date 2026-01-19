package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/parser"
	"github.com/konveyor/test-harness/pkg/targets"
	"github.com/konveyor/test-harness/pkg/util"
	"github.com/konveyor/test-harness/pkg/validator"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

var (
	targetConfigFile string
	targetType       string
	runFilter        string
)

// NewRunCmd creates the run command
func NewRunCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run <test-file-or-directory>",
		Short: "Run test definition(s)",
		Long: `Execute one or more tests and validate their output against expected results.

You can provide either:
  - A specific test file (test.yaml)
  - A directory containing test files (will search recursively)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			log := util.GetLogger()

			// Check if path is a file or directory
			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("failed to stat path: %w", err)
			}

			var testFiles []string
			if info.IsDir() {
				// Find all test.yaml files in directory
				log.Info("Searching for test files", "directory", path)
				testFiles, err = findTestFiles(path)
				if err != nil {
					return fmt.Errorf("failed to find test files: %w", err)
				}

				if len(testFiles) == 0 {
					return fmt.Errorf("no test files found in %s", path)
				}

				log.Info("Found test files", "count", len(testFiles))

				// Filter tests if pattern provided
				if runFilter != "" {
					filtered := []string{}
					for _, tf := range testFiles {
						testName := filepath.Base(filepath.Dir(tf))
						if strings.Contains(testName, runFilter) {
							filtered = append(filtered, tf)
						}
					}
					testFiles = filtered
					log.Info("Filtered test files", "count", len(testFiles), "pattern", runFilter)
				}

				if len(testFiles) == 0 {
					return fmt.Errorf("no test files matched filter: %s", runFilter)
				}
			} else {
				// Single test file
				testFiles = []string{path}
			}

			// Load or create target config once for all tests
			var targetConfig *config.TargetConfig
			if targetConfigFile != "" {
				log.Info("Loading target configuration", "file", targetConfigFile)
				targetConfig, err = config.LoadTargetConfig(targetConfigFile)
				if err != nil {
					return fmt.Errorf("failed to load target config: %w", err)
				}
			} else if targetType != "" {
				// Try to auto-discover config file for the specified target type
				discoveredPath := fmt.Sprintf(".koncur/config/target-%s.yaml", targetType)
				if _, err := os.Stat(discoveredPath); err == nil {
					log.Info("Auto-discovered target configuration", "file", discoveredPath)
					targetConfig, err = config.LoadTargetConfig(discoveredPath)
					if err != nil {
						return fmt.Errorf("failed to load auto-discovered target config: %w", err)
					}
				} else {
					// Create default config for specified type
					targetConfig = &config.TargetConfig{Type: targetType}
				}
			} else {
				// Default to kantra, try to auto-discover first
				discoveredPath := ".koncur/config/target-kantra.yaml"
				if _, err := os.Stat(discoveredPath); err == nil {
					log.Info("Auto-discovered target configuration", "file", discoveredPath)
					targetConfig, err = config.LoadTargetConfig(discoveredPath)
					if err != nil {
						return fmt.Errorf("failed to load auto-discovered target config: %w", err)
					}
				} else {
					// Create default kantra config
					targetConfig = &config.TargetConfig{Type: "kantra"}
				}
			}

			log.Info("Using target", "type", targetConfig.Type)

			// Create target from config
			target, err := targets.NewTarget(targetConfig)
			if err != nil {
				return fmt.Errorf("failed to create target: %w", err)
			}

			// Run all tests
			successCount := 0
			failCount := 0
			skippedCount := 0

			for i, testFile := range testFiles {
				testName := filepath.Base(filepath.Dir(testFile))
				if len(testFiles) > 1 {
					fmt.Printf("\n[%d/%d] Running: %s\n", i+1, len(testFiles), testName)
				}

				// Check if test is marked as skipped
				if isTestSkipped(testFile) {
					color.Yellow("  ⊘ Skipped (marked as SKIPPED in file)")
					skippedCount++
					continue
				}

				// Run single test
				passed, err := runSingleTest(testFile, target, targetConfig)
				if err != nil {
					color.Red("  ✗ Error: %v", err)
					failCount++
					continue
				}

				if passed {
					successCount++
				} else {
					failCount++
				}
			}

			// Print summary if multiple tests
			if len(testFiles) > 1 {
				fmt.Println("\n" + strings.Repeat("=", 60))
				fmt.Printf("Summary: %d total\n", len(testFiles))
				if successCount > 0 {
					color.Green("  ✓ Passed: %d", successCount)
				}
				if skippedCount > 0 {
					color.Yellow("  ⊘ Skipped: %d", skippedCount)
				}
				if failCount > 0 {
					color.Red("  ✗ Failed: %d", failCount)
					return nil
				}
			} else if failCount > 0 {
				return nil
			}

			return nil
		},
	}

	// Flags
	runCmd.Flags().StringVarP(&targetConfigFile, "target-config", "c", "", "Path to target configuration file")
	runCmd.Flags().StringVarP(&targetType, "target", "t", "", "Target type (kantra, tackle-hub, tackle-ui, kai-rpc, vscode)")
	runCmd.Flags().StringVarP(&runFilter, "filter", "f", "", "Filter tests by name pattern (only applies when running a directory)")

	return runCmd
}

// runSingleTest executes a single test and returns whether it passed
func runSingleTest(testFile string, target targets.Target, targetConfig *config.TargetConfig) (bool, error) {
	// Load test definition
	test, err := config.Load(testFile)
	if err != nil {
		return false, fmt.Errorf("failed to load test: %w", err)
	}

	// Validate test definition
	if err := config.Validate(test); err != nil {
		return false, fmt.Errorf("invalid test definition: %w", err)
	}

	// Execute the test
	result, err := target.Execute(context.Background(), test)
	if err != nil {
		return false, fmt.Errorf("execution failed: %w", err)
	}

	// Check exit code
	if result.ExitCode != test.Expect.ExitCode {
		color.Red("  ✗ Exit code mismatch: expected %d, got %d", test.Expect.ExitCode, result.ExitCode)
		return false, nil
	}

	// Parse the output
	actualOutput, err := parser.ParseOutput(result.OutputFile)
	if err != nil {
		return false, fmt.Errorf("failed to parse output: %w", err)
	}

	// Filter actual output to match how expected output is filtered during generation
	filteredActual := parser.FilterRuleSets(actualOutput)

	// Normalize paths in actual output to match expected output format
	normalizedActual, err := normalizeRuleSetPaths(filteredActual, test.GetTestDir())
	if err != nil {
		return false, fmt.Errorf("failed to normalize paths: %w", err)
	}

	// Get target type for validation
	tgtType := ""
	if targetConfig != nil {
		tgtType = targetConfig.Type
	}

	// Validate against expected output using the filtered file
	validation, err := validator.ValidateFiles(test.GetTestDir(), tgtType, normalizedActual, test.Expect.Output.Result)
	if err != nil {
		return false, fmt.Errorf("validation error: %w", err)
	}

	// Report results
	if validation.Passed {
		green := color.New(color.FgGreen, color.Bold)
		green.Printf("  ✓ PASSED")
		fmt.Printf(" - Duration: %s, RuleSets: %d (filtered from %d)\n", result.Duration, len(filteredActual), len(actualOutput))
		return true, nil
	}

	// Test failed
	red := color.New(color.FgRed, color.Bold)
	red.Println("  ✗ FAILED")

	// Print validation errors in a pretty format
	if len(validation.Errors) > 0 {
		fmt.Printf("\n    Found %d validation error(s):\n\n", len(validation.Errors))

		for i, err := range validation.Errors {
			err.Print(i + 1)

			// Add spacing between errors
			if i < len(validation.Errors)-1 {
				fmt.Println()
			}
		}
		fmt.Println()
	}

	return false, nil
}

// normalizeRuleSetPaths normalizes file paths in rulesets to match the expected output format
// This applies the same normalization that saveFilteredOutput does when generating expected output
func normalizeRuleSetPaths(rulesets []konveyor.RuleSet, testDir string) ([]konveyor.RuleSet, error) {
	// Marshal to YAML to normalize paths using string replacement (same approach as generate)
	data, err := yaml.Marshal(rulesets)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rulesets: %w", err)
	}

	yamlStr := string(data)

	// Normalize paths by removing the test directory path
	if testDir != "" {
		yamlStr = strings.ReplaceAll(yamlStr, testDir, "")
	}

	// Normalize Maven repository paths
	if strings.Contains(yamlStr, "/root/.m2/repository") {
		yamlStr = strings.ReplaceAll(yamlStr, "/root/.m2/repository/", "/m2/")
	}
	if strings.Contains(yamlStr, "/cache/m2/") {
		yamlStr = strings.ReplaceAll(yamlStr, "/cache/m2/", "/m2/")
	}

	// Normalize Tackle Hub container paths
	if strings.Contains(yamlStr, "/shared/source/") {
		yamlStr = strings.ReplaceAll(yamlStr, "/shared/source", "/source")
	}
	if strings.Contains(yamlStr, "/opt/input/source") {
		yamlStr = strings.ReplaceAll(yamlStr, "/opt/input/source", "/source")
	}

	// Unmarshal back to get normalized rulesets
	var normalized []konveyor.RuleSet
	if err := yaml.Unmarshal([]byte(yamlStr), &normalized); err != nil {
		return nil, fmt.Errorf("failed to unmarshal normalized rulesets: %w", err)
	}

	return normalized, nil
}
