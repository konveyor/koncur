package validator

import (
	"fmt"
	"maps"
	"reflect"

	"github.com/fatih/color"
	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
)

type tagCompare interface {
	compareTags(expected, actual []string) []ValidationError
}
type violationCompare interface {
	compareViolations(expected, actual map[string]konveyor.Violation) []ValidationError
}
type errorsCompare interface {
	compareErrors(expected, actual map[string]string) []ValidationError
}
type unmatchedCompare interface {
	compareUnmatched(expected, actual []string) []ValidationError
}
type skippedCompare interface {
	compareSkipped(expected, actual []string) []ValidationError
}

// compareViolationsUsing iterates expected/actual violations and delegates detail
// comparison to detailsFn.
func compareViolationsUsing(
	expected, actual map[string]konveyor.Violation,
	detailsFn func(exp, act konveyor.Violation) []ValidationError,
) []ValidationError {
	var errors []ValidationError
	for k, exp := range expected {
		act, exists := actual[k]
		if !exists {
			errors = append(errors, ValidationError{
				Path:     fmt.Sprintf("/%s", k),
				Message:  fmt.Sprintf("Did not find expected violation: %s", k),
				Expected: exp,
			})
			continue
		}
		detailErrors := detailsFn(exp, act)
		for i := range detailErrors {
			detailErrors[i].Path = fmt.Sprintf("/%s%s", k, detailErrors[i].Path)
		}
		errors = append(errors, detailErrors...)
	}
	for k := range actual {
		if _, exists := expected[k]; !exists {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/%s", k),
				Message: fmt.Sprintf("Unexpected violation found: %s", k),
				Actual:  actual[k],
			})
		}
	}
	return errors
}

func findExpectedString(expected string, actual []string) bool {
	for _, a := range actual {
		if expected == a {
			return true
		}
	}
	return false
}

// rulesetFilter is an optional interface that comparers can implement to
// skip expected rulesets that are known to be missing from the target output.
type rulesetFilter interface {
	skipMissingRuleset(rs konveyor.RuleSet) bool
}

type comparer interface {
	tagCompare
	violationCompare
	errorsCompare
	unmatchedCompare
	skippedCompare
}

func getComparer(targetType, testDir string) comparer {
	base := &baseValidator{testDir: testDir}
	switch targetType {
	case "kantra":
		return &kantraValidator{baseValidator: *base}
	case "tackle-hub":
		return &tackleHubValidator{baseValidator: *base}
	case "tackle-ui":
		return &kantraValidator{baseValidator: *base}
	case "kai-rpc":
		return &kaiRpcValidator{baseValidator: *base}
	case "vscode":
		return &kantraValidator{baseValidator: *base}
	}
	return nil
}

// ValidationResult contains the result of validation
type ValidationResult struct {
	Passed bool
	Errors []ValidationError
}

// ValidationError represents a single validation failure
type ValidationError struct {
	Path     string
	Message  string
	Expected any
	Actual   any
}

// Print formats and prints the validation error with colors
func (v ValidationError) Print(index int) {
	// Print error         number and path
	yellow := color.New(color.FgYellow, color.Bold)
	//cyan := color.New(color.FgCyan)
	yellow.Printf("[%d] %s\n", index, v.Path)

	// Print message if present
	if v.Message != "" {
		fmt.Printf("%s\n", v.Message)
	}

	// Print expected vs actual if present
	//	if v.Expected != nil {
	//		cyan.Print("Expected: ")
	//		fmt.Printf("%v\n", v.Expected)
	//	}
	//	if v.Actual != nil {
	//		cyan.Print("Actual:   ")
	//		fmt.Printf("%v\n", v.Actual)
	//	}
}

// Validate performs exact match validation between actual and expected rulesets
// This function now takes file paths and compares the raw YAML content
func Validate(actual, expected []konveyor.RuleSet) (*ValidationResult, error) {
	return ValidateFiles("", "", actual, expected)
}

// ValidateFiles performs exact match validation by comparing YAML files directly
func ValidateFiles(testDir, targetType string, actual, expected []konveyor.RuleSet) (*ValidationResult, error) {
	result := &ValidationResult{
		Passed: true,
		Errors: []ValidationError{},
	}

	errors := []ValidationError{}
	comparer := getComparer(targetType, testDir)

	for _, ers := range expected {
		expectedRulesetName := ers.Name
		if ers.Name == "konveyor-analysis" && targetType == "tackle-hub" {
			expectedRulesetName = "files"
		}

		found := false
		for _, rs := range actual {
			if rs.Name != expectedRulesetName {
				continue
			}
			found = true

			if !maps.Equal(ers.Errors, rs.Errors) {
				errs := comparer.compareErrors(ers.Errors, rs.Errors)
				for i := range errs {
					errs[i].Path = fmt.Sprintf("%s/error%s", rs.Name, errs[i].Path)
				}
				errors = append(errors, errs...)
			}

			if !reflect.DeepEqual(rs.Tags, ers.Tags) {
				errs := comparer.compareTags(ers.Tags, rs.Tags)
				for i := range errs {
					errs[i].Path = fmt.Sprintf("%s/tags%s", rs.Name, errs[i].Path)
				}
				errors = append(errors, errs...)
			}
			if !reflect.DeepEqual(rs.Insights, ers.Insights) {
				errs := comparer.compareViolations(ers.Insights, rs.Insights)
				for i := range errs {
					errs[i].Path = fmt.Sprintf("%s/insights%s", rs.Name, errs[i].Path)
				}
				errors = append(errors, errs...)
			}
			if !reflect.DeepEqual(rs.Violations, ers.Violations) {
				errs := comparer.compareViolations(ers.Violations, rs.Violations)
				for i := range errs {
					errs[i].Path = fmt.Sprintf("%s/violations%s", rs.Name, errs[i].Path)
				}
				errors = append(errors, errs...)
			}
			if !reflect.DeepEqual(rs.Unmatched, ers.Unmatched) {
				errs := comparer.compareUnmatched(ers.Unmatched, rs.Unmatched)
				for i := range errs {
					errs[i].Path = fmt.Sprintf("%s/unmatched%s", rs.Name, errs[i].Path)
				}
				errors = append(errors, errs...)
			}
			if !reflect.DeepEqual(rs.Skipped, ers.Skipped) {
				errs := comparer.compareSkipped(ers.Skipped, rs.Skipped)
				for i := range errs {
					errs[i].Path = fmt.Sprintf("%s/skipped%s", rs.Name, errs[i].Path)
				}
				errors = append(errors, errs...)
			}
			break
		}
		if !found {
			// Check if this comparer allows skipping missing rulesets
			skip := false
			if rf, ok := comparer.(rulesetFilter); ok {
				skip = rf.skipMissingRuleset(ers)
			}
			if !skip {
				errors = append(errors, ValidationError{Path: fmt.Sprintf("ruleset/%s", ers.Name), Message: "Did not find a matching ruleset"})
			}
		}
	}

	expectedRulesetNames := make(map[string]bool)
	for _, ers := range expected {
		expectedRulesetName := ers.Name
		if ers.Name == "konveyor-analysis" && targetType == "tackle-hub" {
			expectedRulesetName = "files"
		}
		expectedRulesetNames[expectedRulesetName] = true
	}
	for _, rs := range actual {
		if !expectedRulesetNames[rs.Name] {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("ruleset/%s", rs.Name),
				Message: fmt.Sprintf("Unexpected ruleset found: %s", rs.Name),
				Actual:  rs.Name,
			})
		}
	}

	// If not equal, generate detailed diff
	result.Passed = len(errors) == 0
	result.Errors = errors

	return result, nil
}
