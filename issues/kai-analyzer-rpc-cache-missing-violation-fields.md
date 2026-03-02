# Cache doesn't preserve Effort, Links, Extras on violations

**Repo:** github.com/konveyor/kai (kai_analyzer_rpc)

## Problem

When violations are stored in the incident cache, only `Description`, `Category`, and `Labels` are copied. The `Effort`, `Links`, and `Extras` fields are dropped.

This causes the RPC response to return violations missing these fields, producing output that differs from kantra (which returns the full engine output directly).

## Root Cause

In `kai_analyzer_rpc/pkg/service/analyzer.go`, the `addRulesetsToCache` method (around line 365) creates a partial copy:

```go
Violation: konveyor.Violation{
    Description: v.Description,
    Category:    v.Category,
    Labels:      v.Labels,
},
```

Missing fields: `Effort`, `Links`, `Extras`

When `createRulesetsFromCache` reconstructs violations, these fields are nil/empty.

## Expected Behavior

The cache should preserve all violation fields so the RPC response matches what the engine produces. kantra returns `effort: 7` for `localhost-jdbc-00002`; kai-rpc returns nil.

## Suggested Fix

Add the missing fields to the cache entry:

```go
Violation: konveyor.Violation{
    Description: v.Description,
    Category:    v.Category,
    Labels:      v.Labels,
    Effort:      v.Effort,
    Links:       v.Links,
    Extras:      v.Extras,
},
```

## Workaround

koncur's `kaiRpcValidator` tolerates `actual.Effort == nil` when `expected.Effort != nil` (see `pkg/validator/kai_rpc_validator.go`).
