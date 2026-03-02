# Analyze RPC doesn't return discovery rulesets or their unmatched rules

**Repo:** github.com/konveyor/kai (kai_analyzer_rpc)

## Problem

The `Analyze` RPC method only returns violation-based rulesets. Discovery rulesets (tags, insights, technology-usage) and their associated unmatched rules are missing from the response, even though they are computed and cached internally.

This causes two downstream issues:
1. Entire rulesets missing: `discovery-rules` and `technology-usage` are absent
2. Incomplete unmatched lists: Rules classified as discovery (e.g., `embedded-cache-libraries-*`) don't appear in unmatched for their parent ruleset

## Root Cause

In `kai_analyzer_rpc/pkg/service/pipe_analyzer.go`, rules are separated into `discoveryRulesets` and `violationRulesets`. Tagging rules and insight-only rules go to discovery:

```go
if interimRule.Perform.Tag != nil && !(interimRule.Perform.Message.Text != nil && interimRule.Effort != nil && *interimRule.Effort != 0) {
    runCacheResetRuleset.Rules = append(runCacheResetRuleset.Rules, interimRule)
}
```

In `analyzer.go`, the `Analyze` method:
1. Runs discovery rulesets and stores results in `a.discoveryCache` (line ~294)
2. Runs violation rulesets and stores in the violation cache (line ~316)
3. Returns ONLY `a.createRulesetsFromCache()` (line 323), which only uses the violation cache

The `discoveryCache` results are never included in the response.

## Impact

For the petclinic test with cloud-readiness rules:
- Expected: 3 rulesets (cloud-readiness, discovery-rules, technology-usage)
- Actual: 1 ruleset (cloud-readiness)
- Expected unmatched: 69 rules (including 16 embedded-cache-libraries-* discovery rules)
- Actual unmatched: 25 rules (only violation rules)

## Expected Behavior

The RPC response should include both discovery and violation rulesets, matching kantra's output. Rulesets with the same name from discovery and violation runs should be merged.

## Suggested Fix

After the violation run in `Analyze()`, merge `discoveryCache` into the response:

```go
response.Rulesets = a.createRulesetsFromCache()
// Merge discovery rulesets
for _, drs := range a.discoveryCache {
    merged := false
    for i, rs := range response.Rulesets {
        if rs.Name == drs.Name {
            // Merge tags, insights, unmatched into existing ruleset
            response.Rulesets[i].Tags = append(rs.Tags, drs.Tags...)
            for k, v := range drs.Insights {
                if response.Rulesets[i].Insights == nil {
                    response.Rulesets[i].Insights = map[string]konveyor.Violation{}
                }
                response.Rulesets[i].Insights[k] = v
            }
            response.Rulesets[i].Unmatched = append(rs.Unmatched, drs.Unmatched...)
            merged = true
            break
        }
    }
    if !merged {
        response.Rulesets = append(response.Rulesets, drs)
    }
}
```

## Workaround

koncur's `kaiRpcValidator`:
- `skipMissingRuleset`: Skips expected rulesets with no violations
- `compareUnmatched`: One-directional check (actual ⊆ expected, tolerates missing expected unmatched)

See `pkg/validator/kai_rpc_validator.go`.
