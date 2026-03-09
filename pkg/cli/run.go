package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/parser"
	"github.com/konveyor/test-harness/pkg/targets"
	"github.com/konveyor/test-harness/pkg/util"
	"github.com/konveyor/test-harness/pkg/validator"
	"github.com/spf13/cobra"
)

var (
	targetConfigFile       string
	targetType             string
	runFilter              string
	outputFormat           string
	outputFile             string
	skipMavenSettingsTests bool
	testArchive            string
)

// NewRunCmd creates the run command
func NewRunCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run [test-file-or-directory]",
		Short: "Run test definition(s)",
		Long: `Execute one or more tests and validate their output against expected results.

You can provide either:
  - A specific test file (test.yaml)
  - A directory containing test files (will search recursively)
  - A test archive (--test-archive path/to/koncur-tests.tar.gz)

When using --test-archive, no positional argument is needed. The archive
will be extracted to a temporary directory and all tests will be run from it.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log := util.GetLogger()
			if val, ok := os.LookupEnv("SKIP_MAVEN"); ok {
				if val != "false" {
					skipMavenSettingsTests = true
				}
			}

			var path string
			if testArchive != "" {
				// Extract test archive to a temp directory
				tmpDir, err := extractTestArchive(testArchive)
				if err != nil {
					return fmt.Errorf("failed to extract test archive: %w", err)
				}
				defer os.RemoveAll(tmpDir)
				path = tmpDir
				log.Info("Using test archive", "archive", testArchive, "extracted", tmpDir)
			} else if len(args) > 0 {
				path = args[0]
			} else {
				return fmt.Errorf("either provide a test path or use --test-archive")
			}

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

			tests := []*config.TestDefinition{}
			skippedCount := 0
			for _, testFile := range testFiles {
				// Load test definition
				test, err := config.Load(testFile)
				if err != nil {
					log.V(1).Info("failed to load test:", "error", err)
					color.Red("  ✗ Failed to load test: %v because: %v", testFile, err)
					continue
				}

				// Validate test definition
				if err := config.Validate(test); err != nil {
					log.V(1).Info("invalid test definition", "error", err)
					color.Red("  ✗ invalid test definition: %v failed because: %v", testFile, err)
					continue
				}
				if test.RequireMavenSettings && skipMavenSettingsTests {
					log.V(1).Info("skipping test because has maven settings requirements")
					color.Yellow("  ⊘ Skipped: %s (maven settings)", test.Name)
					skippedCount += 1
					continue
				}
				if test.Skipped {
					log.V(1).Info("skipping test because marked as skipped")
					color.Yellow("  ⊘ Skipped: %s (marked skip)", test.Name)
					skippedCount += 1
					continue
				}

				tests = append(tests, test)
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
			startTime := time.Now()
			successCount := 0
			failCount := 0
			var allResults []TestResult

			for i, test := range tests {
				if len(tests) > 1 {
					fmt.Printf("\n[%d/%d] Running: %s\n", i+1, len(tests), test.Name)
				}

				// Run single test
				testResult, err := runSingleTest(test, target, targetConfig)
				if err != nil {
					color.Red("  ✗ Error: %v", err)
					failCount++
					if testResult != nil {
						allResults = append(allResults, *testResult)
					}
					continue
				}

				allResults = append(allResults, *testResult)
				if testResult.Status == "passed" {
					successCount++
				} else {
					failCount++
				}
			}

			totalDuration := time.Since(startTime)

			// Create summary
			summary := &TestSummary{
				Total:    len(testFiles),
				Passed:   successCount,
				Failed:   failCount,
				Skipped:  skippedCount,
				Duration: totalDuration.String(),
				Tests:    allResults,
			}

			// Output based on format
			if outputFormat != "console" {
				formatted, err := FormatResults(summary, OutputFormat(outputFormat))
				if err != nil {
					return fmt.Errorf("failed to format results: %w", err)
				}

				// Write to file if specified, otherwise to stdout
				if outputFile != "" {
					if err := os.WriteFile(outputFile, []byte(formatted), 0644); err != nil {
						return fmt.Errorf("failed to write output file: %w", err)
					}
					fmt.Printf("\nTest results written to: %s\n", outputFile)
				}
			}

			// Console format - print summary if multiple tests
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
					for _, tr := range summary.Tests {
						if tr.Status == "failed" {
							color.Red("    ✗ : %s", tr.Name)
						}
					}
				}
			}

			if summary.Failed > 0 {
				os.Exit(1)
			}
			return nil
		},
	}

	// Flags
	runCmd.Flags().StringVarP(&targetConfigFile, "target-config", "c", "", "Path to target configuration file")
	runCmd.Flags().BoolVarP(&skipMavenSettingsTests, "skip-maven", "", false, "Skip the tests that need maven settings files")
	runCmd.Flags().StringVarP(&targetType, "target", "t", "", "Target type (kantra, tackle-hub, tackle-ui, kai-rpc, vscode)")
	runCmd.Flags().StringVarP(&runFilter, "filter", "f", "", "Filter tests by name pattern (only applies when running a directory)")
	runCmd.Flags().StringVarP(&outputFormat, "output-format", "o", "console", "Output format: console, json, yaml, junit")
	runCmd.Flags().StringVar(&outputFile, "output-file", "", "File path to write test results (only for json, yaml, junit formats)")
	runCmd.Flags().StringVar(&testArchive, "test-archive", "", "Path to a test archive (.tar.gz) to extract and run")

	return runCmd
}

// runSingleTest executes a single test and returns the test result
func runSingleTest(test *config.TestDefinition, target targets.Target, targetConfig *config.TargetConfig) (*TestResult, error) {

	// Initialize test result
	testResult := &TestResult{
		Name:   test.Name,
		Status: "unknown",
	}

	startTime := time.Now()

	// Execute the test
	result, err := target.Execute(context.Background(), test)
	if err != nil {
		testResult.Status = "failed"
		testResult.ErrorMessage = fmt.Sprintf("execution failed: %v", err)
		testResult.Duration = time.Since(startTime).String()
		return testResult, fmt.Errorf("execution failed: %w", err)
	}

	testResult.ExitCode = result.ExitCode
	testResult.ExpectedExitCode = test.Expect.ExitCode
	testResult.Duration = result.Duration.String()

	// Check exit code
	if result.ExitCode != test.Expect.ExitCode {
		testResult.Status = "failed"
		testResult.ErrorMessage = fmt.Sprintf("Exit code mismatch: expected %d, got %d", test.Expect.ExitCode, result.ExitCode)
		if outputFormat == "console" {
			color.Red("  ✗ Exit code mismatch: expected %d, got %d", test.Expect.ExitCode, result.ExitCode)
		}
		return testResult, nil
	}

	// Parse the output
	actualOutput, err := parser.ParseOutput(result.OutputFile)
	if err != nil {
		testResult.Status = "failed"
		testResult.ErrorMessage = fmt.Sprintf("failed to parse output: %v", err)
		return testResult, fmt.Errorf("failed to parse output: %w", err)
	}

	// Filter actual output to match how expected output is filtered during generation
	filteredActual := parser.FilterRuleSets(actualOutput)
	testResult.RuleSetsCount = len(filteredActual)
	testResult.FilteredFrom = len(actualOutput)

	// Normalize paths in actual output to match expected output format
	normalizedActual, err := parser.NormalizeRuleSets(filteredActual, test.GetTestDir())
	if err != nil {
		testResult.Status = "failed"
		testResult.ErrorMessage = fmt.Sprintf("failed to normalize paths: %v", err)
		return testResult, fmt.Errorf("failed to normalize paths: %w", err)
	}

	// Get target type for validation
	tgtType := ""
	if targetConfig != nil {
		tgtType = targetConfig.Type
	}

	// Validate against expected output using the filtered file
	validation, err := validator.ValidateFiles(test.GetTestDir(), tgtType, normalizedActual, test.Expect.Output.Result)
	if err != nil {
		testResult.Status = "failed"
		testResult.ErrorMessage = fmt.Sprintf("validation error: %v", err)
		return testResult, fmt.Errorf("validation error: %w", err)
	}

	// Report results
	if validation.Passed {
		testResult.Status = "passed"
		green := color.New(color.FgGreen, color.Bold)
		green.Printf("  ✓ PASSED")
		fmt.Printf(" - Duration: %s, RuleSets: %d (filtered from %d)\n", result.Duration, len(filteredActual), len(actualOutput))
		return testResult, nil
	}

	// Test failed - populate validation errors
	testResult.Status = "failed"
	testResult.ValidationErrors = validation.Errors

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

	return testResult, nil
}

// extractTestArchive extracts a .tar.gz test archive to a temporary directory
// and returns the path to the extracted directory.
func extractTestArchive(archivePath string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open archive %s: %w", archivePath, err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tmpDir, err := os.MkdirTemp("", "koncur-tests-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Sanitize the path to prevent directory traversal
		target := filepath.Join(tmpDir, header.Name)
		rel, err := filepath.Rel(tmpDir, filepath.Clean(target))
		if err != nil || strings.HasPrefix(rel, "..") {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("failed to create directory %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("failed to create parent directory for %s: %w", target, err)
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("failed to create file %s: %w", target, err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("failed to write file %s: %w", target, err)
			}
			outFile.Close()
		}
	}

	return tmpDir, nil
}
