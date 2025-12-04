package cli

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/parser"
	"github.com/konveyor/test-harness/pkg/targets"
	"github.com/konveyor/test-harness/pkg/util"
	"github.com/konveyor/test-harness/pkg/validator"
	"github.com/spf13/cobra"
)

var (
	targetConfigFile string
	targetType       string
)

// NewRunCmd creates the run command
func NewRunCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run <test-file>",
		Short: "Run a test definition",
		Long:  `Execute a test and validate its output against expected results.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			testFile := args[0]
			log := util.GetLogger()

			log.Info("Loading test definition", "file", testFile)

			// Load test definition
			test, err := config.Load(testFile)
			if err != nil {
				return err
			}

			log.Info("Loaded test", "name", test.Name)

			// Validate test definition
			if err := config.Validate(test); err != nil {
				return err
			}

			log.Info("Test definition is valid")

			// Load or create target config
			var targetConfig *config.TargetConfig
			if targetConfigFile != "" {
				log.Info("Loading target configuration", "file", targetConfigFile)
				targetConfig, err = config.LoadTargetConfig(targetConfigFile)
				if err != nil {
					return fmt.Errorf("failed to load target config: %w", err)
				}
			} else if targetType != "" {
				// Create default config for specified type
				targetConfig = &config.TargetConfig{Type: targetType}
			} else {
				// Default to kantra
				targetConfig = &config.TargetConfig{Type: "kantra"}
			}

			log.Info("Using target", "type", targetConfig.Type)

			// Create target from config
			target, err := targets.NewTarget(targetConfig)
			if err != nil {
				return fmt.Errorf("failed to create target: %w", err)
			}

			// Execute the test
			log.Info("Executing analysis", "target", target.Name())
			result, err := target.Execute(context.Background(), test)
			if err != nil {
				return fmt.Errorf("execution failed: %w", err)
			}

			// Check exit code
			if result.ExitCode != test.Expect.ExitCode {
				return fmt.Errorf("exit code mismatch: expected %d, got %d", test.Expect.ExitCode, result.ExitCode)
			}

			log.Info("Analysis completed successfully", "duration", result.Duration)

			// Parse the output
			log.Info("Parsing output", "file", result.OutputFile)
			actualOutput, err := parser.ParseOutput(result.OutputFile)
			if err != nil {
				return fmt.Errorf("failed to parse output: %w", err)
			}

			log.Info("Output parsed successfully", "rulesets", len(actualOutput))

			// Filter actual output to match how expected output is filtered during generation
			// This removes empty rulesets (no violations, insights, or tags)
			filteredActual := parser.FilterRuleSets(actualOutput)
			log.Info("Filtered actual output", "original", len(actualOutput), "filtered", len(filteredActual))

			// Validate against expected output
			log.Info("Validating output against expected results")
			validation, err := validator.Validate(filteredActual, test.Expect.Output.Result)
			if err != nil {
				return fmt.Errorf("validation error: %w", err)
			}

			// Report results
			if validation.Passed {
				green := color.New(color.FgGreen, color.Bold)
				green.Println("✓ Test PASSED")
				fmt.Printf("  Test: %s\n", test.Name)
				fmt.Printf("  Duration: %s\n", result.Duration)
				fmt.Printf("  RuleSets: %d (filtered from %d)\n", len(filteredActual), len(actualOutput))
				return nil
			}

			// Test failed
			red := color.New(color.FgRed, color.Bold)
			red.Println("✗ Test FAILED")
			fmt.Printf("  Test: %s\n", test.Name)
			fmt.Println("\nDifferences:")
			fmt.Println(validation.Diff)

			return fmt.Errorf("test failed: output does not match expected")
		},
	}

	// Flags
	runCmd.Flags().StringVarP(&targetConfigFile, "target-config", "c", "", "Path to target configuration file")
	runCmd.Flags().StringVarP(&targetType, "target", "t", "", "Target type (kantra, tackle-hub, tackle-ui, kai-rpc, vscode)")

	return runCmd
}
