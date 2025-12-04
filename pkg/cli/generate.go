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
	"github.com/spf13/cobra"
	yaml2 "gopkg.in/yaml.v2"
	"gopkg.in/yaml.v3"
)

var (
	testDir        string
	outputDir      string
	generateFilter string
	dryRun         bool
	targetTypeGen  string
)

// NewGenerateCmd creates the generate command
func NewGenerateCmd() *cobra.Command {
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate expected outputs for tests",
		Long: `Generate expected outputs by running tests and capturing their actual results.
This command will:
  1. Find all test.yaml files in the specified directory
  2. Execute each test using the specified target (default: kantra)
  3. Save the actual output as the expected output for each test

This is useful when:
  - Creating new tests and need to capture baseline outputs
  - Updating tests after tool behavior changes
  - Regenerating outputs after fixing test definitions`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := util.GetLogger()

			// Find all test.yaml files
			log.Info("Searching for test files", "directory", testDir)
			testFiles, err := findTestFiles(testDir)
			if err != nil {
				return fmt.Errorf("failed to find test files: %w", err)
			}

			if len(testFiles) == 0 {
				return fmt.Errorf("no test files found in %s", testDir)
			}

			log.Info("Found test files", "count", len(testFiles))

			// Filter tests if pattern provided
			if generateFilter != "" {
				filtered := []string{}
				for _, tf := range testFiles {
					testName := filepath.Base(filepath.Dir(tf))
					if strings.Contains(testName, generateFilter) {
						filtered = append(filtered, tf)
					}
				}
				testFiles = filtered
				log.Info("Filtered test files", "count", len(testFiles), "pattern", generateFilter)
			}

			if len(testFiles) == 0 {
				return fmt.Errorf("no test files matched filter: %s", generateFilter)
			}

			// Process each test
			successCount := 0
			failCount := 0
			skippedCount := 0

			for i, testFile := range testFiles {
				testName := filepath.Base(filepath.Dir(testFile))
				fmt.Printf("\n[%d/%d] Processing: %s\n", i+1, len(testFiles), testName)

				// Load test definition (skip loading expected output since we're generating it)
				test, err := config.LoadWithOptions(testFile, true)
				if err != nil {
					color.Red("  ✗ Failed to load: %v", err)
					failCount++
					continue
				}

				// Check if test is marked as skipped
				if isTestSkipped(testFile) {
					color.Yellow("  ⊘ Skipped (marked as SKIPPED in file)")
					skippedCount++
					continue
				}

				// Validate test definition (skip expected output validation since we're generating it)
				if err := validateTestForGeneration(test); err != nil {
					color.Red("  ✗ Invalid test definition: %v", err)
					failCount++
					continue
				}

				// Create target config
				targetConfig := &config.TargetConfig{Type: targetTypeGen}

				// Create target
				target, err := targets.NewTarget(targetConfig)
				if err != nil {
					color.Red("  ✗ Failed to create target: %v", err)
					failCount++
					continue
				}

				if dryRun {
					color.Cyan("  ⇢ Would execute: %s", target.Name())
					successCount++
					continue
				}

				// Execute the test
				log.Info("Executing analysis", "test", testName, "target", target.Name())
				result, err := target.Execute(context.Background(), test)
				if err != nil {
					color.Red("  ✗ Execution failed: %v", err)
					failCount++
					continue
				}

				color.Blue("  ⟳ Analysis completed (exit code: %d, duration: %s)", result.ExitCode, result.Duration)

				// Parse the output
				actualOutput, err := parser.ParseOutput(result.OutputFile)
				if err != nil {
					color.Red("  ✗ Failed to parse output: %v", err)
					failCount++
					continue
				}

				log.Info("Output parsed", "rulesets", len(actualOutput))

				// Filter rulesets to only include those with violations, insights, or tags
				filteredOutput := parser.FilterRuleSets(actualOutput)
				log.Info("Filtered output", "original", len(actualOutput), "filtered", len(filteredOutput))

				// Update test to use file-based expectation
				test.Expect.ExitCode = result.ExitCode
				test.Expect.Output.Result = nil // Clear inline expectation

				// Save the filtered output.yaml file to the test directory
				testDir := filepath.Dir(testFile)
				expectedOutputFile := filepath.Join(testDir, "expected-output.yaml")

				// Save the filtered output as YAML
				if err := saveFilteredOutput(filteredOutput, expectedOutputFile); err != nil {
					color.Red("  ✗ Failed to save filtered output: %v", err)
					failCount++
					continue
				}

				test.Expect.Output.File = "expected-output.yaml"

				// Save updated test definition
				if err := saveSimpleTestDefinition(testFile, test); err != nil {
					color.Red("  ✗ Failed to save: %v", err)
					failCount++
					continue
				}

				color.Green("  ✓ Generated and saved expected output (%d rulesets, %d filtered)", len(filteredOutput), len(actualOutput)-len(filteredOutput))
				successCount++
			}

			// Print summary
			fmt.Println("\n" + strings.Repeat("=", 60))
			fmt.Printf("Summary: %d total\n", len(testFiles))
			if successCount > 0 {
				color.Green("  ✓ Success: %d", successCount)
			}
			if skippedCount > 0 {
				color.Yellow("  ⊘ Skipped: %d", skippedCount)
			}
			if failCount > 0 {
				color.Red("  ✗ Failed: %d", failCount)
				return fmt.Errorf("failed to generate outputs for %d tests", failCount)
			}

			return nil
		},
	}

	// Flags
	generateCmd.Flags().StringVarP(&testDir, "test-dir", "d", "./tests", "Directory containing test definitions")
	generateCmd.Flags().StringVarP(&generateFilter, "filter", "f", "", "Filter tests by name pattern")
	generateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without executing")
	generateCmd.Flags().StringVarP(&targetTypeGen, "target", "t", "kantra", "Target type to use (kantra, tackle-hub, tackle-ui, kai-rpc, vscode)")

	return generateCmd
}

// findTestFiles recursively finds all test.yaml files in the given directory
func findTestFiles(dir string) ([]string, error) {
	var testFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Base(path) == "test.yaml" {
			testFiles = append(testFiles, path)
		}
		return nil
	})

	return testFiles, err
}

// isTestSkipped checks if the test file contains a SKIPPED marker in the first few lines
func isTestSkipped(testFile string) bool {
	content, err := os.ReadFile(testFile)
	if err != nil {
		return false
	}

	// Check first 500 bytes for SKIPPED marker
	searchContent := string(content)
	if len(searchContent) > 500 {
		searchContent = searchContent[:500]
	}

	return strings.Contains(searchContent, "SKIPPED:") || strings.Contains(searchContent, "# SKIPPED")
}

// validateTestForGeneration validates a test but skips expected output validation
// since we're about to generate the expected output
func validateTestForGeneration(test *config.TestDefinition) error {
	// Basic validation of required fields
	if test.Name == "" {
		return fmt.Errorf("test name is required")
	}
	if test.Analysis.Application == "" {
		return fmt.Errorf("analysis application is required")
	}
	if test.Analysis.AnalysisMode == "" {
		return fmt.Errorf("analysis mode is required")
	}
	return nil
}

// saveSimpleTestDefinition saves a simplified test definition
// This avoids the circular reference issue in RuleSet.MarshalYAML
func saveSimpleTestDefinition(testFile string, test *config.TestDefinition) error {
	// Create a simplified structure without the Result field
	type SimpleExpectedOutput struct {
		File string `yaml:"file,omitempty"`
	}

	type SimpleExpectConfig struct {
		ExitCode int                  `yaml:"exitCode"`
		Output   SimpleExpectedOutput `yaml:"output"`
	}

	type SimpleTestDefinition struct {
		Name        string                `yaml:"name"`
		Description string                `yaml:"description,omitempty"`
		Analysis    config.AnalysisConfig `yaml:"analysis"`
		Timeout     *config.Duration      `yaml:"timeout,omitempty"`
		WorkDir     string                `yaml:"workDir,omitempty"`
		Expect      SimpleExpectConfig    `yaml:"expect"`
	}

	simpleTest := SimpleTestDefinition{
		Name:        test.Name,
		Description: test.Description,
		Analysis:    test.Analysis,
		Timeout:     test.Timeout,
		WorkDir:     test.WorkDir,
		Expect: SimpleExpectConfig{
			ExitCode: test.Expect.ExitCode,
			Output: SimpleExpectedOutput{
				File: test.Expect.Output.File,
			},
		},
	}

	// Marshal the simplified test
	updatedContent, err := yaml.Marshal(simpleTest)
	if err != nil {
		return fmt.Errorf("failed to marshal test: %w", err)
	}

	// Write to file
	if err := os.WriteFile(testFile, updatedContent, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	err = os.WriteFile(dst, input, 0644)
	if err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}

// saveFilteredOutput saves the filtered rulesets to a YAML file
// Uses yaml.v2 to match analyzer-lsp's marshalling behavior and avoid circular reference issues
func saveFilteredOutput(rulesets []konveyor.RuleSet, path string) error {
	// Use yaml.v2 because konveyor types were designed for v2
	// v3 has different MarshalYAML behavior that causes infinite recursion
	data, err := yaml2.Marshal(rulesets)
	if err != nil {
		return fmt.Errorf("failed to marshal rulesets: %w", err)
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
