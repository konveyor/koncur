package validator

import (
	"fmt"
	"maps"
	"slices"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/konveyor/test-harness/pkg/util"
)

type incidentField int

const (
	URI incidentField = iota
	LINE_NUMBER
	CODE_SNIP
	MESSAGE
	NONE
)

func (i incidentField) String() string {
	switch i {
	case URI:
		return "uri"
	case LINE_NUMBER:
		return "line number"
	case CODE_SNIP:
		return "code snip"
	case MESSAGE:
		return "message"
	}
	return ""
}

type baseValidator struct {
	testDir string
}

// normalizeURI removes ephemeral storage paths (containers, temp dirs)
// and normalizes to a consistent format for comparison
func (b *baseValidator) normalizeURI(uri string) string {
	return uri
	// Match any path up to and including java-bin-NUMBERS, replace with normalized path
	//	javaBinPattern := regexp.MustCompile(`file.*/java-bin-\d+/`)
	//	normalized := javaBinPattern.ReplaceAllString(uri, "file:///source/")
	//
	//	// Additional normalizations for other container/cache paths
	//	normalized = strings.ReplaceAll(normalized, "/root/.m2/repository/", "/m2/")
	//	normalized = strings.ReplaceAll(normalized, "/cache/m2/", "/m2/")
	//	normalized = strings.ReplaceAll(normalized, "/shared/source/", "/source/")
	//	normalized = strings.ReplaceAll(normalized, "/opt/input/source/", "/source/")
	//
	//	return normalized
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
		faild_fields := map[incidentField]*struct{}{}
		for _, ai := range actual.Incidents {
			if ok, field_faild := b.incidentsMatch(i, ai); ok {
				found = true
				break
			} else {
				faild_fields[field_faild] = nil
			}
		}
		if !found {
			field_error := slices.Sorted(maps.Keys(faild_fields))[len(faild_fields)-1]
			normalizedURI := b.normalizeURI(string(i.URI))
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Did not find expected incident:  %s:%d failed to match on: %s", normalizedURI, lineNumberOrZero(i.LineNumber), field_error.String()),
			})
		}
	}

	for _, ai := range actual.Incidents {
		found := false
		faild_fields := map[incidentField]*struct{}{}
		for _, i := range expected.Incidents {
			if ok, field_faild := b.incidentsMatch(i, ai); ok {
				found = true
				break
			} else {
				faild_fields[field_faild] = nil
			}
		}
		if !found {
			normalizedURI := b.normalizeURI(string(ai.URI))
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Unexpected incident found: %s:%d", normalizedURI, lineNumberOrZero(ai.LineNumber)),
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

func (b *baseValidator) incidentsMatch(expected, actual konveyor.Incident) (bool, incidentField) {
	if string(expected.URI) != string(actual.URI) {
		return false, URI
	}
	expectedLN := lineNumberOrZero(expected.LineNumber)
	actualLN := lineNumberOrZero(actual.LineNumber)
	if expectedLN != actualLN {
		return false, LINE_NUMBER
	}
	logger := util.GetLogger()
	if expected.Message != actual.Message {
		logger.Info("messages don't match", "expected", expected.Message, "actual", actual.Message)
		return false, MESSAGE
	}
	// Here three is a problem where the variables may not be the exact same.
	// To compare we would have to know what is being returned and parse it.
	// Because of this, if the uri line number and code are the same
	// Then we can reasonably be sure the incident is the same.

	//	if strings.TrimSpace(expected.CodeSnip) != "" && strings.TrimSpace(expected.CodeSnip) != strings.TrimSpace(actual.CodeSnip) {
	//		logger.Info("code snip's don't match", "expected", expected.CodeSnip, "actual", actual.CodeSnip)
	//		return false, CODE_SNIP
	//	}

	//	if len(expected.Variables) > 0 && !reflect.DeepEqual(expected.Variables, actual.Variables) {
	//		log.Info("here", "vars", actual.Variables)
	//		return false
	//	}

	return true, NONE
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
