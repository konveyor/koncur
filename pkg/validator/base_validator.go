package validator

import (
	"fmt"
	"reflect"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
)

type baseValidator struct {
	testDir string
}

func (b *baseValidator) compareTags(expected, actual []string) []ValidationError {
	var errors []ValidationError
	for _, exp := range expected {
		if !findExpectedString(exp, actual) {
			errors = append(errors, ValidationError{
				Path:     fmt.Sprintf("/%s", exp),
				Message:  fmt.Sprintf("Did not find expected tag: %s", exp),
				Expected: exp,
			})
		}
	}
	for _, act := range actual {
		if !findExpectedString(act, expected) {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/%s", act),
				Message: fmt.Sprintf("Unexpected tag found: %s", act),
				Actual:  act,
			})
		}
	}

	return errors
}

func (b *baseValidator) compareViolations(expected, actual map[string]konveyor.Violation) []ValidationError {
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

		detailErrors := b.compareViolationDetails(exp, act)
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

func (b *baseValidator) compareViolationDetails(expected, actual konveyor.Violation) []ValidationError {
	var errors []ValidationError

	if actual.Category != nil && expected.Category != nil && *expected.Category != *actual.Category {
		errors = append(errors, ValidationError{
			Message: fmt.Sprintf("Did not find expected category: %v", expected.Category),
		})
	}
	if (expected.Effort != nil && actual.Effort != nil) && (*expected.Effort != *actual.Effort) {
		errors = append(errors, ValidationError{
			Message: fmt.Sprintf("Did not find expected effort: %v", expected.Effort),
		})
	}
	// Handle Links
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
	// Handle Incidents - collect all missing incidents and report as one error
	for _, i := range expected.Incidents {
		found := false
		for _, ai := range actual.Incidents {
			if b.incidentsMatch(i, ai) {
				found = true
				break
			}
		}
		if !found {
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Did not find expected incident: %s:%d", i.URI, i.LineNumber),
			})
		}
	}

	for _, ai := range actual.Incidents {
		found := false
		for _, i := range expected.Incidents {
			if b.incidentsMatch(i, ai) {
				found = true
				break
			}
		}
		if !found {
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Unexpected incident found: %s:%d", ai.URI, ai.LineNumber),
			})
		}
	}

	return errors
}

func lineNumberOrZero(ln *int) int {
	if ln != nil {
		return *ln
	}
	return 0
}

func (b *baseValidator) incidentsMatch(expected, actual konveyor.Incident) bool {
	if strings.TrimSpace(expected.CodeSnip) != "" && strings.TrimSpace(expected.CodeSnip) != strings.TrimSpace(actual.CodeSnip) {
		return false
	}
	if string(expected.URI) != string(actual.URI) {
		return false
	}
	if expected.Message != actual.Message {
		return false
	}
	expectedLN := lineNumberOrZero(expected.LineNumber)
	actualLN := lineNumberOrZero(actual.LineNumber)
	if expectedLN != actualLN {
		return false
	}

	if len(expected.Variables) > 0 && !reflect.DeepEqual(expected.Variables, actual.Variables) {
		return false
	}

	return true
}

func (b *baseValidator) compareErrors(expected, actual map[string]string) []ValidationError {
	var errors []ValidationError
	for k, exp := range expected {
		act, exists := actual[k]
		if !exists || exp != act {
			errors = append(errors, ValidationError{
				Path:     fmt.Sprintf("/%s", k),
				Message:  fmt.Sprintf("Did not find expected error: %s", exp),
				Expected: exp,
			})
		}
	}
	for k := range actual {
		if _, exists := expected[k]; !exists {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/%s", k),
				Message: fmt.Sprintf("Unexpected error found: %s", k),
				Actual:  actual[k],
			})
		}
	}

	return errors
}

func (b *baseValidator) compareUnmatched(expected, actual []string) []ValidationError {
	var errors []ValidationError
	for _, exp := range expected {
		if !findExpectedString(exp, actual) {
			errors = append(errors, ValidationError{
				Path:     fmt.Sprintf("/%s", exp),
				Message:  fmt.Sprintf("Did not find expected unmatched rule: %s", exp),
				Expected: exp,
			})
		}
	}
	for _, act := range actual {
		if !findExpectedString(act, expected) {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/%s", act),
				Message: fmt.Sprintf("Unexpected unmatched rule found: %s", act),
				Actual:  act,
			})
		}
	}

	return errors
}

func (b *baseValidator) compareSkipped(expected, actual []string) []ValidationError {
	var errors []ValidationError
	for _, exp := range expected {
		if !findExpectedString(exp, actual) {
			errors = append(errors, ValidationError{
				Path:     fmt.Sprintf("/%s", exp),
				Message:  fmt.Sprintf("Did not find expected skipped rule: %s", exp),
				Expected: exp,
			})
		}
	}
	for _, act := range actual {
		if !findExpectedString(act, expected) {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/%s", act),
				Message: fmt.Sprintf("Unexpected skipped rule found: %s", act),
				Actual:  act,
			})
		}
	}

	return errors
}
