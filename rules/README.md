# Custom Rules for Testing

This directory contains custom analysis rules used by koncur tests.

## Rules

### jmh-gradle-annotation-state-test-rule.yaml
- **Purpose**: Detects usage of `@State` annotations from JMH (Java Microbenchmark Harness)
- **Pattern**: `org.openjdk.jmh.annotations.State`
- **Category**: mandatory
- **Effort**: 1
- **Used by**: `tests/jmh-gradle/`

### jmh-gradle-serializable-test-rule.yaml
- **Purpose**: Detects references to `java.io.Serializable` interface
- **Pattern**: `java.io.Serializable`
- **Category**: mandatory
- **Effort**: 1
- **Used by**: `tests/jmh-gradle-opensource/`

## Usage in Tests

Rules are referenced using Git URLs to ensure they work with both Kantra and Tackle Hub:

```yaml
rules:
  - https://github.com/konveyor/koncur#<commit-sha>/rules/jmh-gradle-annotation-state-test-rule.yaml
```

This allows both local (Kantra) and remote (Hub) analysis to fetch and use the custom rules.

## Stability

Rules should be pinned to a specific commit SHA or release tag for test stability:
- Use commit SHA for immutability
- Or create a release tag (e.g., `test-rules-v1.0`)

## Adding New Rules

1. Add your rule YAML file to this directory
2. Update this README with documentation
3. Commit and push the changes
4. Reference it in your test using the Git URL with commit SHA/tag
