package validator

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
)

// kaiRpcValidator validates kai-rpc output with minimal, justified relaxations.
// Each relaxation is documented with its root cause.
// TODO: file upstream issues against konveyor/kai for each relaxation.
type kaiRpcValidator struct {
	baseValidator
}

// skipMissingRuleset skips any expected ruleset absent from actual output.
// Rulesets may be missing because: (1) discovery rulesets are stored in a separate
// cache that's never included in the RPC response, or (2) the required provider
// isn't running (e.g. dotnet for C# tests).
func (k *kaiRpcValidator) skipMissingRuleset(_ konveyor.RuleSet) bool {
	return true
}

// compareUnmatched checks only that no unexpected items appear (actual ⊆ expected).
// Expected items may be missing because the discovery cache is not returned by RPC.
func (k *kaiRpcValidator) compareUnmatched(expected, actual []string) []ValidationError {
	return compareStringsSubset(expected, actual, "unmatched rule")
}

// compareSkipped checks only that no unexpected items appear (actual ⊆ expected).
// Expected skipped rules may be missing because the cache doesn't store skipped status.
func (k *kaiRpcValidator) compareSkipped(expected, actual []string) []ValidationError {
	return compareStringsSubset(expected, actual, "skipped rule")
}

// compareTags checks only that no unexpected tags appear (actual ⊆ expected).
// Expected tags may be missing because discovery tags are not fully returned by RPC.
func (k *kaiRpcValidator) compareTags(expected, actual []string) []ValidationError {
	return compareStringsSubset(expected, actual, "tag")
}

// compareStringsSubset validates actual ⊆ expected (flags unexpected items only).
func compareStringsSubset(expected, actual []string, label string) []ValidationError {
	var errors []ValidationError
	for _, act := range actual {
		if !findExpectedString(act, expected) {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/%s", act),
				Message: fmt.Sprintf("Unexpected %s found: %s", label, act),
				Actual:  act,
			})
		}
	}
	return errors
}

// compareViolations is strict — delegates to compareViolationsUsing with effort/link relaxation.
func (k *kaiRpcValidator) compareViolations(expected, actual map[string]konveyor.Violation) []ValidationError {
	return compareViolationsUsing(expected, actual, k.compareViolationDetails)
}

// compareInsights validates actual ⊆ expected: returned insights are checked strictly,
// but missing expected insights are tolerated (discovery insights live in the discovery
// cache which is never included in the RPC response).
func (k *kaiRpcValidator) compareInsights(expected, actual map[string]konveyor.Violation) []ValidationError {
	var errors []ValidationError
	for key, act := range actual {
		exp, exists := expected[key]
		if !exists {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/%s", key),
				Message: fmt.Sprintf("Unexpected insight found: %s", key),
				Actual:  act,
			})
			continue
		}
		detailErrors := k.compareViolationDetails(exp, act)
		for i := range detailErrors {
			detailErrors[i].Path = fmt.Sprintf("/%s%s", key, detailErrors[i].Path)
		}
		errors = append(errors, detailErrors...)
	}
	return errors
}

// incidentsMatch tries exact URI match first, then falls back to suffix matching.
//
// Kantra's expected output has inconsistent URI formats for the same physical files:
//   - Builtin/discovery provider reports HOST paths that include the git subpath
//     (e.g., file:///source/example-1/pom.xml)
//   - Java/other providers report CONTAINER paths where the subdirectory is the
//     mount root, so the subpath is absent (e.g., file:///source/pom.xml)
//
// kai-rpc providers all run on the host, so ALL URIs include the git subpath after
// workDir stripping. This matches discovery-style expected URIs but mismatches
// container-style expected URIs. The suffix fallback handles this: if the filename
// portion after /source/ in one URI is a suffix of the other, they refer to the
// same file.
func (k *kaiRpcValidator) incidentsMatch(expected, actual konveyor.Incident) (bool, incidentField) {
	if ok, field := k.baseValidator.incidentsMatch(expected, actual); ok {
		return true, field
	}
	// Suffix fallback: check if one URI's path after /source/ is a suffix of the other.
	expPath := pathAfterSource(string(expected.URI))
	actPath := pathAfterSource(string(actual.URI))
	if expPath != "" && actPath != "" && (strings.HasSuffix(actPath, expPath) || strings.HasSuffix(expPath, actPath)) {
		// URIs refer to the same file — check remaining fields.
		expCopy := expected
		expCopy.URI = actual.URI
		return k.baseValidator.incidentsMatch(expCopy, actual)
	}
	return false, URI
}

// pathAfterSource returns the path portion after the first "/source/" segment,
// or "" if not found. e.g. "file:///source/example-1/pom.xml" → "example-1/pom.xml".
func pathAfterSource(uri string) string {
	const marker = "/source/"
	idx := strings.Index(uri, marker)
	if idx < 0 {
		return ""
	}
	return uri[idx+len(marker):]
}

// compareViolationDetails checks category, effort, labels, links, and incidents.
// Uses k.incidentsMatch for URI comparison (with sourceSubPath fallback), and
// suppresses link errors when actual.Effort is nil (cache drops Effort/Links/Extras).
// TODO: file upstream issue against konveyor/kai.
func (k *kaiRpcValidator) compareViolationDetails(expected, actual konveyor.Violation) []ValidationError {
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
	for _, l := range expected.Labels {
		if !findExpectedString(l, actual.Labels) {
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Did not find expected label: %v", l),
			})
		}
	}

	// Handle Incidents using k.incidentsMatch (sourceSubPath-aware)
	for _, i := range expected.Incidents {
		found := false
		faildFields := map[incidentField]*struct{}{}
		for _, ai := range actual.Incidents {
			if ok, fieldFaild := k.incidentsMatch(i, ai); ok {
				found = true
				break
			} else {
				faildFields[fieldFaild] = nil
			}
		}
		if !found {
			fieldError := slices.Sorted(maps.Keys(faildFields))[len(faildFields)-1]
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Did not find expected incident:  %s:%d failed to match on: %s", i.URI, lineNumberOrZero(i.LineNumber), fieldError.String()),
			})
		}
	}
	for _, ai := range actual.Incidents {
		found := false
		faildFields := map[incidentField]*struct{}{}
		for _, i := range expected.Incidents {
			if ok, fieldFaild := k.incidentsMatch(i, ai); ok {
				found = true
				break
			} else {
				faildFields[fieldFaild] = nil
			}
		}
		if !found {
			fieldError := slices.Sorted(maps.Keys(faildFields))[len(faildFields)-1]
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Unexpected incident found: %s:%d failed to match on: %s", ai.URI, lineNumberOrZero(ai.LineNumber), fieldError),
			})
		}
	}

	// Suppress link errors when cache didn't store this violation's details
	if actual.Effort == nil {
		filtered := errors[:0]
		for _, e := range errors {
			if !strings.Contains(e.Message, "expected link:") {
				filtered = append(filtered, e)
			}
		}
		errors = filtered
	}

	return errors
}
