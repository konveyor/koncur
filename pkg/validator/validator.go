package validator

import (
	"fmt"
	"os"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/pmezard/go-difflib/difflib"
)

// ValidationResult contains the result of validation
type ValidationResult struct {
	Passed bool
	Errors []ValidationError
	Diff   string
}

// ValidationError represents a single validation failure
type ValidationError struct {
	Path     string
	Message  string
	Expected interface{}
	Actual   interface{}
}

// Validate performs exact match validation between actual and expected rulesets
// This function now takes file paths and compares the raw YAML content
func Validate(actual, expected []konveyor.RuleSet) (*ValidationResult, error) {
	return ValidateFiles("", "", "", actual, expected)
}

// ValidateFiles performs exact match validation by comparing YAML files directly
func ValidateFiles(actualFile, expectedFile, testDir string, actual, expected []konveyor.RuleSet) (*ValidationResult, error) {
	result := &ValidationResult{
		Passed: true,
		Errors: []ValidationError{},
	}

	var actualYAML, expectedYAML string
	var err error

	// Read actual output YAML
	if actualFile != "" {
		data, err := os.ReadFile(actualFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read actual output file: %w", err)
		}
		actualYAML = string(data)
	} else {
		// If no file provided, we can't compare without marshaling which causes stack overflow
		return nil, fmt.Errorf("actualFile path is required for validation")
	}

	// Read expected output YAML
	if expectedFile != "" {
		data, err := os.ReadFile(expectedFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read expected output file: %w", err)
		}
		expectedYAML = string(data)
	} else {
		return nil, fmt.Errorf("expectedFile path is required for validation")
	}

	// Normalize YAML strings by removing test directory paths
	actualNormalized := normalizeYAMLPaths(actualYAML, testDir)
	expectedNormalized := normalizeYAMLPaths(expectedYAML, testDir)

	// Quick check: compare normalized YAML strings
	if actualNormalized == expectedNormalized {
		return result, nil
	}

	// If not equal, generate detailed diff
	result.Passed = false

	// Generate unified diff using normalized YAML
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(expectedNormalized),
		B:        difflib.SplitLines(actualNormalized),
		FromFile: "Expected",
		ToFile:   "Actual",
		Context:  3,
	}

	diffText, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return nil, fmt.Errorf("failed to generate diff: %w", err)
	}

	result.Diff = diffText

	// Add a general error
	result.Errors = append(result.Errors, ValidationError{
		Path:     "rulesets",
		Message:  "Output does not match expected result",
		Expected: expected,
		Actual:   actual,
	})

	return result, nil
}

// normalizeYAMLPaths normalizes paths in YAML by removing test directory paths
func normalizeYAMLPaths(yamlStr, testDir string) string {
	// Replace the test directory path with empty string
	if testDir != "" {
		yamlStr = strings.ReplaceAll(yamlStr, testDir, "")
	}
	return yamlStr
}
