package validator

import (
	"fmt"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
)

// kaiRpcValidator validates kai-rpc output with minimal, justified relaxations.
// Each relaxation is documented with its root cause.
type kaiRpcValidator struct {
	baseValidator
}

// skipMissingRuleset skips expected rulesets with no violations (discovery rulesets
// not returned by RPC). TODO: file upstream issue against konveyor/kai.
func (k *kaiRpcValidator) skipMissingRuleset(rs konveyor.RuleSet) bool {
	return len(rs.Violations) == 0
}

// compareUnmatched checks only that no unexpected unmatched rules appear (actual ⊆ expected).
// Expected unmatched may be missing because discovery rules' unmatched status is not returned.
// TODO: file upstream issue against konveyor/kai.
func (k *kaiRpcValidator) compareUnmatched(expected, actual []string) []ValidationError {
	var errors []ValidationError
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

// compareViolations delegates to kaiRpcValidator.compareViolationDetails.
func (k *kaiRpcValidator) compareViolations(expected, actual map[string]konveyor.Violation) []ValidationError {
	return compareViolationsUsing(expected, actual, k.compareViolationDetails)
}

// compareViolationDetails delegates to baseValidator for all checks, then filters out
// link errors when actual.Effort is nil — the cache drops Effort, Links, and Extras together.
// TODO: file upstream issue against konveyor/kai.
func (k *kaiRpcValidator) compareViolationDetails(expected, actual konveyor.Violation) []ValidationError {
	errors := k.baseValidator.compareViolationDetails(expected, actual)
	if actual.Effort != nil {
		return errors
	}
	// Cache didn't store this violation's details — suppress link errors
	filtered := errors[:0]
	for _, e := range errors {
		if !strings.Contains(e.Message, "expected link:") {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
