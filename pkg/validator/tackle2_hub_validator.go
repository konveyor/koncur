package validator

import (
	"fmt"
	"path/filepath"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
)

type tackleHubValidator struct {
	baseValidator
}

// Don't compare - hub doesn't store this info in the API AFAICT
func (t *tackleHubValidator) compareUnmatched(expected, actual []string) []ValidationError {
	return nil
}

func (t *tackleHubValidator) compareSkipped(expected, actual []string) []ValidationError {
	return nil
}

func (t *tackleHubValidator) compareTags(expected, actual []string) []ValidationError {
	return nil
}

func (t *tackleHubValidator) compareViolations(expected, actual map[string]konveyor.Violation) []ValidationError {
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

		detailErrors := t.compareViolationDetails(exp, act)
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

func (t *tackleHubValidator) compareViolationDetails(expected, actual konveyor.Violation) []ValidationError {
	var errors []ValidationError
	skipForInsight := expected.Effort == nil
	if !skipForInsight && (expected.Effort != nil && actual.Effort != nil) && (*expected.Effort != *actual.Effort) {
		errors = append(errors, ValidationError{
			Message: fmt.Sprintf("Did not find expected effort: %v", expected.Effort),
		})
	}
	if !skipForInsight && actual.Category != nil && expected.Category != nil && *expected.Category != *actual.Category {
		errors = append(errors, ValidationError{
			Message: fmt.Sprintf("Did not find expected category: %v", expected.Category),
		})
	}

	// Handle Links
	if !skipForInsight {
		for _, l := range expected.Links {
			found := false
			for _, al := range actual.Links {
				if l.Title == al.Title && l.URL == al.URL {
					found = true
					break
				}
			}
			if !found {
				errors = append(errors, ValidationError{
					Message: fmt.Sprintf("Did not find expected link: %v", l),
				})
			}
		}
		// Handle Labels
		for _, l := range expected.Labels {
			if !findExpectedString(l, actual.Labels) {
				errors = append(errors, ValidationError{
					Message: fmt.Sprintf("Did not find expected label: %v", l),
				})
			}
		}
	}
	// Handle Incidents
	for _, i := range expected.Incidents {
		found := false
		for _, ai := range actual.Incidents {
			if t.incidentsMatch(i, ai) {
				found = true
				break
			}
		}
		if !found && !skipForInsight {
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Did not find expected incident: %s:%d", i.URI, lineNumberOrZero(i.LineNumber)),
			})
		}
	}
	for _, ai := range actual.Incidents {
		found := false
		for _, i := range expected.Incidents {
			if t.incidentsMatch(i, ai) {
				found = true
				break
			}
		}
		if !found && !skipForInsight {
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Unexpected incident found: %s:%d", ai.URI, lineNumberOrZero(ai.LineNumber)),
				Actual:  ai,
			})
		}
	}

	return errors
}

func (t *tackleHubValidator) incidentsMatch(expected, actual konveyor.Incident) bool {
	// For code snips, there is no way to configure them
	// So for tackle2Hub we are going to ignore code snips
	if string(expected.URI) != "" && string(actual.URI) != "" {
		if expected.URI != actual.URI {
			pathToTest, err := filepath.Rel("/source", expected.URI.Filename())
			if err != nil {
				return false
			}
			if !strings.Contains(actual.URI.Filename(), pathToTest) {
				return false
			}
		}
	}
	if expected.Message != actual.Message {
		return false
	}
	if expected.LineNumber != nil && actual.LineNumber != nil && *expected.LineNumber != *actual.LineNumber {
		return false
	}

	return true
}
