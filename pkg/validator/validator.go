package validator

import (
	"fmt"
	"reflect"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/pmezard/go-difflib/difflib"
	"gopkg.in/yaml.v3"
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
func Validate(actual, expected []konveyor.RuleSet) (*ValidationResult, error) {
	result := &ValidationResult{
		Passed: true,
		Errors: []ValidationError{},
	}

	// Quick check: use DeepEqual for exact match
	if reflect.DeepEqual(actual, expected) {
		return result, nil
	}

	// If not equal, generate detailed diff
	result.Passed = false

	// Marshal both to YAML for human-readable diff
	actualYAML, err := yaml.Marshal(actual)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal actual output: %w", err)
	}

	expectedYAML, err := yaml.Marshal(expected)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal expected output: %w", err)
	}

	// Generate unified diff
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(expectedYAML)),
		B:        difflib.SplitLines(string(actualYAML)),
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
