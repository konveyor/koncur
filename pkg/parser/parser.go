package parser

import (
	"fmt"
	"os"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"gopkg.in/yaml.v3"
)

// ParseOutput reads and parses the analyzer output.yaml file
func ParseOutput(outputFile string) ([]konveyor.RuleSet, error) {
	data, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read output file %s: %w", outputFile, err)
	}

	var rulesets []konveyor.RuleSet
	if err := yaml.Unmarshal(data, &rulesets); err != nil {
		return nil, fmt.Errorf("failed to parse output YAML: %w", err)
	}

	return rulesets, nil
}

// FilterRuleSets filters out rulesets that don't have violations, insights, or tags
// This is used to normalize output for comparison, removing empty rulesets
func FilterRuleSets(rulesets []konveyor.RuleSet) []konveyor.RuleSet {
	var filtered []konveyor.RuleSet
	for _, rs := range rulesets {
		// Keep rulesets that have violations, insights, or tags
		if len(rs.Violations) > 0 || len(rs.Insights) > 0 || len(rs.Tags) > 0 {
			filtered = append(filtered, rs)
		}
	}
	return filtered
}
