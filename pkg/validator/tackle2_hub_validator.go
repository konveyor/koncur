package validator

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/konveyor/test-harness/pkg/util"
)

type tackleHubValidator struct {
	baseValidator
}

// Don't compare hub doesn't store this info in the API AFAICT
func (t *tackleHubValidator) compareUnmatched(expected string, actual []string) (*ValidationError, bool) {
	return nil, false
}

func (t *tackleHubValidator) compareSkipped(expected string, actual []string) (*ValidationError, bool) {
	return nil, false
}

// Incidents are not saved for insights for tags only AFAICT
func (t *tackleHubValidator) compareViolation(expected, actual konveyor.Violation) ([]ValidationError, bool) {
	log := util.GetLogger()
	validationError := []ValidationError{}
	if reflect.DeepEqual(actual, konveyor.Violation{}) {
		return []ValidationError{
			{
				Message:  "No matching violation found",
				Expected: expected,
			},
		}, true
	}
	skipForInsight := expected.Effort == nil
	if !skipForInsight && (expected.Effort != nil && actual.Effort != nil) && (*expected.Effort != *actual.Effort) {
		log.Info("checking effort failed", "expected", *expected.Effort, "actual", *actual.Effort)
		validationError = append(validationError, ValidationError{
			Path:    "",
			Message: fmt.Sprintf("Did not find expected effort: %v", expected.Effort),
		})
	}
	if !skipForInsight && actual.Category != nil && *expected.Category != *actual.Category {
		validationError = append(validationError, ValidationError{
			Path:    "",
			Message: fmt.Sprintf("Did not find expected category: %v", expected.Category),
		})
	}

	// Handle Links
	if !skipForInsight {
		for _, l := range expected.Links {
			found := false
			for _, al := range actual.Links {
				if l.Title == al.Title && l.URL == al.Title {
					found = true
					break
				}
			}
			if !found && !skipForInsight {
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
	}
	// Handle Incidents
	for _, i := range expected.Incidents {
		found := false
		for _, ai := range actual.Incidents {
			// For code snips, there is no way to confifgure them
			// So for tackle2Hub we are going to ignore code snips
			if i.URI != "" && ai.URI != "" {
				// We need to handle the normalization to get to the actual source code
				pathToTest, err := filepath.Rel("/source", i.URI.Filename())
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
			if i.LineNumber != nil && ai.LineNumber != nil && *i.LineNumber != *ai.LineNumber {
				log.Info("checking LineNumber failed", "expected", *i.LineNumber, "actual", *ai.LineNumber)
				continue
			}
			found = true
		}

		if !found && !skipForInsight {
			var message string
			if i.LineNumber != nil {
				message = fmt.Sprintf("Did not find expected incident: %s:%d", i.URI, *i.LineNumber)
			} else {
				message = fmt.Sprintf("Did not find expected incident: %s:0", i.URI)
			}

			validationError = append(validationError, ValidationError{
				Path:    "",
				Message: message,
			})
		}
	}

	return validationError, len(validationError) != 0
}

func (t *tackleHubValidator) compareTag(expected string, actual []string) (*ValidationError, bool) {
	// This is going to be hacky, need to follow category and all of that to get the full tag value, maybe.
	log := util.GetLogger()
	for _, a := range actual {
		if strings.Contains(expected, a) {
			return nil, false
		}
	}
	log.Info("did not find", "expected", expected, "actuals", actual)
	return &ValidationError{
		Path:     "",
		Message:  fmt.Sprintf("Did not find expected tag: %s", expected),
		Expected: expected,
		Actual:   nil,
	}, true
}
