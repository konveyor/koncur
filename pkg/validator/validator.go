package validator

import (
	"fmt"
	"maps"
	"reflect"

	"github.com/fatih/color"
	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/konveyor/test-harness/pkg/util"
)

type tagCompare interface {
	compareTag(expected string, actual []string) (*ValidationError, bool)
}
type violationCompare interface {
	compareViolation(expected, actual konveyor.Violation) ([]ValidationError, bool)
}
type errorsCompare interface {
	compareErrors(expected, actual string) (*ValidationError, bool)
}
type unmatchedCompare interface {
	compareUnmatched(expected string, actual []string) (*ValidationError, bool)
}
type skippedCompare interface {
	compareSkipped(expected string, actual []string) (*ValidationError, bool)
}

func findExpectedString(expected string, actual []string) bool {
	for _, a := range actual {
		if expected == a {
			return true
		}
	}
	return false
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
		return &kantraValidator{baseValidator: *base}
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

	log := util.GetLogger()
	errors := []ValidationError{}
	comparer := getComparer(targetType, testDir)

	for _, ers := range expected {
		found := false
		for _, rs := range actual {
			if rs.Name != ers.Name {
				log.Info("not_found", "rs_name", rs.Name, "ers_name", ers.Name)
				continue
			}
			found = true
			log.Info("found", "rs_name", rs.Name, "ers_name", ers.Name)

			if !maps.Equal(ers.Errors, rs.Errors) {
				for k, eerr := range ers.Errors {
					if err, ok := comparer.compareErrors(eerr, rs.Errors[k]); ok {
						err.Path = fmt.Sprintf("%s/error/%s", rs.Name, k)
						errors = append(errors, *err)
					}
				}
			}

			if !reflect.DeepEqual(rs.Tags, ers.Tags) {
				for _, erstags := range ers.Tags {
					if err, ok := comparer.compareTag(erstags, rs.Tags); ok {
						err.Path = fmt.Sprintf("%s/tags/%s", rs.Name, erstags)
						errors = append(errors, *err)
					}
				}
			}
			if !reflect.DeepEqual(rs.Insights, ers.Insights) {
				for k, ersinsights := range ers.Insights {
					if err, ok := comparer.compareViolation(ersinsights, rs.Insights[k]); ok {

						newMessage := "Did not find Insights\n\t"
						for _, e := range err {
							newMessage = fmt.Sprintf("%s\n\t%s", newMessage, e.Message)
						}

						errors = append(errors, ValidationError{
							Path:     fmt.Sprintf("%s/insights/%s", rs.Name, k),
							Message:  newMessage,
							Expected: ersinsights,
						})
					}
				}

			}
			if !reflect.DeepEqual(rs.Violations, ers.Violations) {
				for k, ersinsights := range ers.Violations {
					if err, ok := comparer.compareViolation(ersinsights, rs.Violations[k]); ok {

						newMessage := "Did not find violations\n\t"
						for _, e := range err {
							newMessage = fmt.Sprintf("%s\n\t%s", newMessage, e.Message)
						}

						errors = append(errors, ValidationError{
							Path:     fmt.Sprintf("%s/violation/%s", rs.Name, k),
							Message:  newMessage,
							Expected: ersinsights,
						})
					}
				}
			}
			if !reflect.DeepEqual(rs.Unmatched, ers.Unmatched) {
				for _, ersunmatched := range ers.Unmatched {
					if err, ok := comparer.compareUnmatched(ersunmatched, rs.Unmatched); ok {
						err.Path = fmt.Sprintf("%s/unmatched/%s", rs.Name, ersunmatched)
						errors = append(errors, *err)
					}
				}
			}
			if !reflect.DeepEqual(rs.Skipped, ers.Skipped) {
				for _, ersskipped := range ers.Skipped {
					if err, ok := comparer.compareSkipped(ersskipped, rs.Skipped); ok {
						err.Path = fmt.Sprintf("%s/skipped/%s", rs.Name, ersskipped)
						errors = append(errors, *err)
					}
				}
			}
			break
		}
		if !found {
			log.Info("not_found_error", "ers_name", ers.Name)
			errors = append(errors, ValidationError{Path: fmt.Sprintf("ruleset/%s", ers.Name), Message: "Did not find a matching ruleset"})
		}
	}

	// If not equal, generate detailed diff
	result.Passed = len(errors) == 0
	result.Errors = errors

	return result, nil
}
