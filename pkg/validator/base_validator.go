package validator

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
)

type baseValidator struct {
	testDir string
}

func (b *baseValidator) compareTag(expected string, actual []string) (*ValidationError, bool) {
	if findExpectedString(expected, actual) {
		return nil, false
	}
	// Didn't find expected tag
	return &ValidationError{
		Path:     "",
		Message:  fmt.Sprintf("Did not find expected tag: %s", expected),
		Expected: expected,
		Actual:   nil,
	}, true
}

func (b *baseValidator) compareViolation(expected, actual konveyor.Violation) ([]ValidationError, bool) {
	validationError := []ValidationError{}
	if reflect.DeepEqual(actual, konveyor.Violation{}) {
		return []ValidationError{
			{
				Message: "Unable to find violation",
			},
		}, true
	}

	if expected.Category != actual.Category {
		validationError = append(validationError, ValidationError{
			Path:    "",
			Message: fmt.Sprintf("Did not find expected category: %v", expected.Category),
		})
	}
	if expected.Effort != actual.Effort {
		validationError = append(validationError, ValidationError{
			Path:    "",
			Message: fmt.Sprintf("Did not find expected effort: %v", expected.Effort),
		})
	}
	// Handle Links
	for _, l := range expected.Links {
		found := false
		for _, al := range actual.Links {
			if l.Title == al.Title && l.URL == al.Title {
				found = true
				break
			}
		}
		if !found {
			validationError = append(validationError, ValidationError{
				Path:    "",
				Message: fmt.Sprintf("Did not find expected links: %v", l),
			})
		}
	}
	// Handle Labels
	for _, l := range expected.Labels {
		if findExpectedString(l, actual.Labels) {
			continue
		}
		validationError = append(validationError, ValidationError{
			Path:    "",
			Message: fmt.Sprintf("Did not find expected label: %v", l),
		})
	}
	// Handle Incidents - collect all missing incidents and report as one error
	for _, i := range expected.Incidents {
		found := false
		for _, ai := range actual.Incidents {
			if strings.TrimSpace(i.CodeSnip) != strings.TrimSpace(ai.CodeSnip) {
				continue
			}
			// Skip URI comparison if either URI is empty
			if string(i.URI) == "" || string(ai.URI) == "" {
				if string(i.URI) != string(ai.URI) {
					continue
				}
			} else {
				pathToTest, err := filepath.Rel(filepath.Join(b.testDir, "source"), i.URI.Filename())
				if err != nil {
					break
				}
				if !strings.Contains(ai.URI.Filename(), pathToTest) {
					continue
				}
			}
			if i.Message != ai.Message {
				continue
			}
			if i.LineNumber != ai.LineNumber {
				continue
			}
			if !reflect.DeepEqual(i.Variables, ai.Variables) {
				continue
			}
			found = true
		}
		if !found {
			validationError = append(validationError, ValidationError{
				Path:    "",
				Message: fmt.Sprintf("Did not find expected incident: %s:%d", i.URI, i.LineNumber),
			})
		}
	}

	return validationError, len(validationError) != 0
}

func (b *baseValidator) compareErrors(expected, actual string) (*ValidationError, bool) {
	if expected != actual {
		return &ValidationError{
			Path:     "",
			Message:  fmt.Sprintf("Did not find expected error: %s", expected),
			Expected: expected,
			Actual:   nil,
		}, true
	}
	return nil, false
}

func (b *baseValidator) compareUnmatched(expected string, actual []string) (*ValidationError, bool) {
	if findExpectedString(expected, actual) {
		return nil, false
	}
	// Didn't find expected tag
	return &ValidationError{
		Path:     "",
		Message:  fmt.Sprintf("Did not find expected unmatched rule: %s", expected),
		Expected: expected,
		Actual:   nil,
	}, true
}

func (b *baseValidator) compareSkipped(expected string, actual []string) (*ValidationError, bool) {
	if findExpectedString(expected, actual) {
		return nil, false
	}
	// Didn't find expected tag
	return &ValidationError{
		Path:     "",
		Message:  fmt.Sprintf("Did not find expected skipped rule: %s", expected),
		Expected: expected,
		Actual:   nil,
	}, true
}
