package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	cleanAll    bool
	cleanDryRun bool
)

// NewCleanCmd creates the clean command
func NewCleanCmd() *cobra.Command {
	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean up old test run outputs",
		Long: `Clean up the .koncur/output directory, keeping only the latest run for each test.

By default, keeps the most recent run for each test and deletes older ones.
Use --all to remove all output directories.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			outputBaseDir := ".koncur/output"

			// Check if directory exists
			if _, err := os.Stat(outputBaseDir); os.IsNotExist(err) {
				fmt.Println("Nothing to clean - .koncur/output directory doesn't exist")
				return nil
			}

			if cleanAll {
				return cleanAllOutputs(outputBaseDir)
			}

			return cleanOldOutputs(outputBaseDir)
		},
	}

	cleanCmd.Flags().BoolVar(&cleanAll, "all", false, "Remove all output directories (not just old ones)")
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "Show what would be deleted without actually deleting")

	return cleanCmd
}

// cleanAllOutputs removes all output directories
func cleanAllOutputs(outputBaseDir string) error {
	if cleanDryRun {
		fmt.Println("Dry run mode - would delete:")
		fmt.Printf("  %s/\n", outputBaseDir)
		return nil
	}

	fmt.Printf("Removing all outputs: %s/\n", outputBaseDir)
	err := os.RemoveAll(outputBaseDir)
	if err != nil {
		return fmt.Errorf("failed to remove directory: %w", err)
	}

	color.Green("✓ All outputs cleaned")
	return nil
}

// cleanOldOutputs keeps only the latest run for each test
func cleanOldOutputs(outputBaseDir string) error {
	// Read all entries in the output directory
	entries, err := os.ReadDir(outputBaseDir)
	if err != nil {
		return fmt.Errorf("failed to read output directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("Nothing to clean - output directory is empty")
		return nil
	}

	// Group directories by test name (everything before the timestamp)
	testRuns := make(map[string][]string)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		testName := extractTestName(dirName)
		if testName == "" {
			continue // Skip if we can't parse the test name
		}

		testRuns[testName] = append(testRuns[testName], dirName)
	}

	// For each test, sort by timestamp and keep only the latest
	var toDelete []string
	var toKeep []string

	for _, runs := range testRuns {
		if len(runs) <= 1 {
			toKeep = append(toKeep, runs...)
			continue // Nothing to clean for this test
		}

		// Sort runs by directory name (which includes timestamp)
		sort.Strings(runs)

		// Keep the last one (most recent), delete the rest
		toKeep = append(toKeep, runs[len(runs)-1])
		toDelete = append(toDelete, runs[:len(runs)-1]...)
	}

	if len(toDelete) == 0 {
		fmt.Println("Nothing to clean - only latest runs exist")
		return nil
	}

	// Show what will be deleted
	fmt.Printf("Found %d old run(s) to clean up:\n", len(toDelete))
	for _, dir := range toDelete {
		fmt.Printf("  - %s\n", dir)
	}

	fmt.Printf("\nKeeping %d latest run(s):\n", len(toKeep))
	for _, dir := range toKeep {
		fmt.Printf("  + %s\n", dir)
	}

	if cleanDryRun {
		color.Cyan("\nDry run mode - no files were deleted")
		return nil
	}

	// Delete old directories
	deletedCount := 0
	for _, dir := range toDelete {
		dirPath := filepath.Join(outputBaseDir, dir)
		err := os.RemoveAll(dirPath)
		if err != nil {
			color.Red("✗ Failed to delete %s: %v", dir, err)
			continue
		}
		deletedCount++
	}

	color.Green("\n✓ Cleaned up %d old run(s)", deletedCount)
	return nil
}

// extractTestName extracts the test name from a directory name
// Expected format: {TestName}-{YYYYMMDD-HHMMSS}
func extractTestName(dirName string) string {
	// Find the last occurrence of a timestamp pattern (YYYYMMDD-HHMMSS)
	// Everything before the last "-YYYYMMDD-HHMMSS" is the test name
	parts := strings.Split(dirName, "-")
	if len(parts) < 3 {
		return "" // Not a valid test run directory
	}

	// The last two parts should be date and time (e.g., "20251204-004136")
	// Check if the last part looks like a time (HHMMSS - 6 digits)
	lastPart := parts[len(parts)-1]
	if len(lastPart) != 6 {
		return ""
	}

	// Check if the second-to-last part looks like a date (YYYYMMDD - 8 digits)
	datePart := parts[len(parts)-2]
	if len(datePart) != 8 {
		return ""
	}

	// Test name is everything except the last two parts
	return strings.Join(parts[:len(parts)-2], "-")
}
