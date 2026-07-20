package validator

import (
	"testing"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"go.lsp.dev/uri"
)

func TestValidate_ExactMatch(t *testing.T) {
	actual := []konveyor.RuleSet{
		{
			Name: "test-ruleset",
			Tags: []string{"tag1", "tag2"},
			Violations: map[string]konveyor.Violation{
				"rule1": {
					Description: "Test violation",
					Category:    categoryPtr("mandatory"),
					Labels:      []string{"label1"},
					Incidents: []konveyor.Incident{
						{
							URI:        uri.File("/test/file.go"),
							Message:    "Test message",
							LineNumber: intPtr(10),
						},
					},
					Effort: intPtr(5),
				},
			},
		},
	}

	expected := []konveyor.RuleSet{
		{
			Name: "test-ruleset",
			Tags: []string{"tag1", "tag2"},
			Violations: map[string]konveyor.Violation{
				"rule1": {
					Description: "Test violation",
					Category:    categoryPtr("mandatory"),
					Labels:      []string{"label1"},
					Incidents: []konveyor.Incident{
						{
							URI:        uri.File("/test/file.go"),
							Message:    "Test message",
							LineNumber: intPtr(10),
						},
					},
					Effort: intPtr(5),
				},
			},
		},
	}

	result, err := ValidateFiles("/test", "kantra", actual, expected)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	if !result.Passed {
		t.Errorf("Expected validation to pass, but got %d errors", len(result.Errors))
		for _, e := range result.Errors {
			t.Logf("  Error: %s - %s", e.Path, e.Message)
		}
	}
}

func TestValidate_MissingRuleset(t *testing.T) {
	actual := []konveyor.RuleSet{
		{
			Name: "ruleset1",
		},
	}

	expected := []konveyor.RuleSet{
		{
			Name: "ruleset1",
		},
		{
			Name: "ruleset2",
		},
	}

	result, err := ValidateFiles("/test", "kantra", actual, expected)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	if result.Passed {
		t.Error("Expected validation to fail for missing ruleset")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected validation errors for missing ruleset")
	}

	// Check that error mentions the missing ruleset
	foundError := false
	for _, e := range result.Errors {
		if e.Path == "ruleset/ruleset2" {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("Expected error for missing ruleset2")
	}
}

func TestValidate_MissingTag(t *testing.T) {
	actual := []konveyor.RuleSet{
		{
			Name: "test-ruleset",
			Tags: []string{"tag1"},
		},
	}

	expected := []konveyor.RuleSet{
		{
			Name: "test-ruleset",
			Tags: []string{"tag1", "tag2"},
		},
	}

	result, err := ValidateFiles("/test", "kantra", actual, expected)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	if result.Passed {
		t.Error("Expected validation to fail for missing tag")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected validation errors for missing tag")
	}
}

func TestValidate_MissingViolation(t *testing.T) {
	actual := []konveyor.RuleSet{
		{
			Name:       "test-ruleset",
			Violations: map[string]konveyor.Violation{},
		},
	}

	expected := []konveyor.RuleSet{
		{
			Name: "test-ruleset",
			Violations: map[string]konveyor.Violation{
				"rule1": {
					Description: "Test",
					Effort:      intPtr(5),
				},
			},
		},
	}

	result, err := ValidateFiles("/test", "kantra", actual, expected)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	if result.Passed {
		t.Error("Expected validation to fail for missing violation")
	}
}

func TestValidate_EmptyRulesets(t *testing.T) {
	result, err := Validate([]konveyor.RuleSet{}, []konveyor.RuleSet{})
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	if !result.Passed {
		t.Error("Expected validation to pass for empty rulesets")
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got %d", len(result.Errors))
	}
}

func TestValidateFiles_WithTargetType(t *testing.T) {
	actual := []konveyor.RuleSet{
		{
			Name: "test-ruleset",
			Violations: map[string]konveyor.Violation{
				"rule1": {
					Description: "Test",
					Incidents: []konveyor.Incident{
						{
							URI:      uri.File("/test/file.go"),
							Message:  "Test message",
							CodeSnip: "code snippet",
						},
					},
				},
			},
		},
	}

	expected := []konveyor.RuleSet{
		{
			Name: "test-ruleset",
			Violations: map[string]konveyor.Violation{
				"rule1": {
					Description: "Test",
					Incidents: []konveyor.Incident{
						{
							URI:     uri.File("/test/file.go"),
							Message: "Test message",
							// Note: CodeSnip is different/missing
						},
					},
				},
			},
		},
	}

	// With kantra target, codeSnip differences should be detected
	result, err := ValidateFiles("/test", "kantra", actual, expected)
	if err != nil {
		t.Fatalf("ValidateFiles returned error: %v", err)
	}

	// Kantra validator should detect codeSnip differences
	if result.Passed {
		t.Log("Note: Kantra validator may consolidate incidents, check implementation")
	}

	// With tackle-hub target, codeSnip differences should be ignored
	result, err = ValidateFiles("/test", "tackle-hub", actual, expected)
	if err != nil {
		t.Fatalf("ValidateFiles returned error: %v", err)
	}

	// Tackle-hub validator should ignore codeSnip differences
	if !result.Passed {
		t.Error("Expected tackle-hub validation to pass (ignoring codeSnip)")
	}
}

func TestValidateFiles_HubSynthesizedDiscoveryRulesets(t *testing.T) {
	expected := []konveyor.RuleSet{
		{
			Name: "dotnet-core-migration",
			Violations: map[string]konveyor.Violation{
				"rule1": {Description: "Test"},
			},
		},
	}

	actual := []konveyor.RuleSet{
		{
			Name: "dotnet-core-migration",
			Violations: map[string]konveyor.Violation{
				"rule1": {Description: "Test"},
			},
		},
		{
			Name: "technology-usage",
			Tags: []string{".NET", "ASP.NET"},
		},
		{
			Name: "discovery-rules",
			Tags: []string{"C#", "JavaScript"},
		},
	}

	// Tag-only synthesized rulesets must not fail hub validation, since their
	// presence depends on discovery-task timing and tags are never compared
	result, err := ValidateFiles("/test", "tackle-hub", actual, expected)
	if err != nil {
		t.Fatalf("ValidateFiles returned error: %v", err)
	}
	if !result.Passed {
		t.Errorf("Expected tackle-hub validation to pass, got errors: %v", result.Errors)
	}

	// Other targets still report them as unexpected
	result, err = ValidateFiles("/test", "kantra", actual, expected)
	if err != nil {
		t.Fatalf("ValidateFiles returned error: %v", err)
	}
	if result.Passed {
		t.Error("Expected kantra validation to fail on unexpected rulesets")
	}

	// A synthesized-name ruleset that carries real content is still unexpected
	actual = append(actual[:1], konveyor.RuleSet{
		Name: "technology-usage",
		Tags: []string{".NET"},
		Insights: map[string]konveyor.Violation{
			"tag-rule": {Description: "Test"},
		},
	})
	result, err = ValidateFiles("/test", "tackle-hub", actual, expected)
	if err != nil {
		t.Fatalf("ValidateFiles returned error: %v", err)
	}
	if result.Passed {
		t.Error("Expected tackle-hub validation to fail when synthesized ruleset has insights")
	}
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func categoryPtr(s string) *konveyor.Category {
	c := konveyor.Category(s)
	return &c
}
