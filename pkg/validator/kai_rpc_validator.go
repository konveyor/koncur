package validator

import (
	"fmt"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
)

// kaiRpcValidator validates kai-rpc output with minimal, justified relaxations.
//
// Kai-rpc runs the same analyzer-lsp engine as kantra, so output should be near-identical.
// Each relaxation here is documented with its root cause and linked to a draft issue
// for upstream fixes in kai-analyzer-rpc.
type kaiRpcValidator struct {
	baseValidator
}

// skipMissingRuleset skips expected rulesets that only contain tags/insights (no violations).
//
// Root cause: analyzer.go:323 only returns createRulesetsFromCache() which reconstructs
// from the violation cache. Discovery rulesets (discoveryCache) are populated but never
// included in the RPC response.
//
// Upstream issue: issues/kai-analyzer-rpc-discovery-rulesets-not-returned.md
func (k *kaiRpcValidator) skipMissingRuleset(rs konveyor.RuleSet) bool {
	return len(rs.Violations) == 0
}

// compareUnmatched uses one-directional validation: every actual unmatched rule must
// exist in expected (no unexpected unmatched), but expected unmatched rules may be
// missing from actual.
//
// Root cause: Rules are separated into discoveryRulesets and violationRulesets in
// pipe_analyzer.go. Tagging/discovery rules (e.g., embedded-cache-libraries-*) are
// classified as discovery and run separately. Their unmatched status is tracked in
// discoveryCache, which is never returned. So only violation-rule unmatched are reported.
//
// Upstream issue: issues/kai-analyzer-rpc-discovery-rulesets-not-returned.md
func (k *kaiRpcValidator) compareUnmatched(expected, actual []string) []ValidationError {
	var errors []ValidationError
	// Check that no unexpected unmatched rules appear
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

// compareViolations delegates to kaiRpcValidator.compareViolationDetails
// (needed because Go embedded struct methods don't dispatch virtually).
func (k *kaiRpcValidator) compareViolations(expected, actual map[string]konveyor.Violation) []ValidationError {
	return compareViolationsUsing(expected, actual, k.compareViolationDetails)
}

// compareViolationDetails is strict like baseValidator, except it tolerates
// actual.Effort being nil when expected.Effort is set.
//
// Root cause: The incident cache (analyzer.go:365-369) only stores Description,
// Category, and Labels on violations — omitting Effort, Links, and Extras.
// When createRulesetsFromCache reconstructs violations, these fields are nil.
//
// Upstream issue: issues/kai-analyzer-rpc-cache-missing-violation-fields.md
func (k *kaiRpcValidator) compareViolationDetails(expected, actual konveyor.Violation) []ValidationError {
	var errors []ValidationError

	// Effort: strict when both present, tolerate actual nil (cache doesn't store it)
	if expected.Effort != nil && actual.Effort != nil && *expected.Effort != *actual.Effort {
		errors = append(errors, ValidationError{
			Message: fmt.Sprintf("Did not find expected effort: %v got %v", *expected.Effort, *actual.Effort),
		})
	}

	// Category: strict
	if actual.Category != nil && expected.Category != nil && *expected.Category != *actual.Category {
		errors = append(errors, ValidationError{
			Message: fmt.Sprintf("Did not find expected category: %v got %v", *expected.Category, *actual.Category),
		})
	}

	// Links: strict (tolerate missing when actual.Effort is nil, since cache drops Links too)
	if actual.Effort != nil {
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
	}

	// Labels: strict
	for _, l := range expected.Labels {
		if !findExpectedString(l, actual.Labels) {
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Did not find expected label: %v", l),
			})
		}
	}

	// Incidents: strict (URIs are now normalized via workDir stripping).
	// Uses baseValidator.incidentsMatch for exact matching.
	for _, i := range expected.Incidents {
		found := false
		for _, ai := range actual.Incidents {
			if ok, _ := k.incidentsMatch(i, ai); ok {
				found = true
				break
			}
		}
		if !found {
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Did not find expected incident: %s:%d", i.URI, lineNumberOrZero(i.LineNumber)),
			})
		}
	}
	for _, ai := range actual.Incidents {
		found := false
		for _, i := range expected.Incidents {
			if ok, _ := k.incidentsMatch(i, ai); ok {
				found = true
				break
			}
		}
		if !found {
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Unexpected incident found: %s:%d", ai.URI, lineNumberOrZero(ai.LineNumber)),
				Actual:  ai,
			})
		}
	}

	return errors
}
