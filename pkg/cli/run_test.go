package cli

import (
	"reflect"
	"testing"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
)

func TestCollectAnalysisErrors(t *testing.T) {
	tests := []struct {
		name     string
		rulesets []konveyor.RuleSet
		want     []string
	}{
		{
			name:     "no rulesets",
			rulesets: nil,
			want:     nil,
		},
		{
			name: "clean analysis has no errors",
			rulesets: []konveyor.RuleSet{
				{Name: "discovery-rules", Violations: map[string]konveyor.Violation{"rule-1": {}}},
			},
			want: nil,
		},
		{
			name: "error-only ruleset is reported",
			rulesets: []konveyor.RuleSet{
				{Name: "component-changes", Errors: map[string]string{
					"component-changes-00008": "could not parse xpath",
				}},
			},
			want: []string{"component-changes.component-changes-00008: could not parse xpath"},
		},
		{
			name: "errors across rulesets are sorted",
			rulesets: []konveyor.RuleSet{
				{Name: "zeta", Errors: map[string]string{"z-2": "boom", "z-1": "bang"}},
				{Name: "alpha", Errors: map[string]string{"a-1": "kaboom"}},
			},
			want: []string{
				"alpha.a-1: kaboom",
				"zeta.z-1: bang",
				"zeta.z-2: boom",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectAnalysisErrors(tt.rulesets)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectAnalysisErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}
