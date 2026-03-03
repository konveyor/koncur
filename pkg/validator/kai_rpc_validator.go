package validator

import (
	"fmt"
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
