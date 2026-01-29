package parser

import (
	"os"
	"path/filepath"
	"testing"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"go.lsp.dev/uri"
	"gopkg.in/yaml.v2"
)

func TestParseOutput(t *testing.T) {
	tests := []struct {
		name        string
		setupFile   func(t *testing.T) string
		expectError bool
		validate    func(t *testing.T, result []konveyor.RuleSet)
	}{
		{
			name: "valid YAML file",
			setupFile: func(t *testing.T) string {
				tmpDir := t.TempDir()
				outputFile := filepath.Join(tmpDir, "output.yaml")
				rulesets := []konveyor.RuleSet{
					{
						Name:        "test-ruleset",
						Description: "Test Description",
						Tags:        []string{"tag1", "tag2"},
					},
				}
				data, err := yaml.Marshal(rulesets)
				if err != nil {
					t.Fatalf("Failed to marshal test data: %v", err)
				}
				if err := os.WriteFile(outputFile, data, 0644); err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
				return outputFile
			},
			expectError: false,
			validate: func(t *testing.T, result []konveyor.RuleSet) {
				if len(result) != 1 {
					t.Errorf("Expected 1 ruleset, got %d", len(result))
				}
				if result[0].Name != "test-ruleset" {
					t.Errorf("Expected ruleset name 'test-ruleset', got '%s'", result[0].Name)
				}
				if len(result[0].Tags) != 2 {
					t.Errorf("Expected 2 tags, got %d", len(result[0].Tags))
				}
			},
		},
		{
			name: "file not found",
			setupFile: func(t *testing.T) string {
				return "/nonexistent/file.yaml"
			},
			expectError: true,
		},
		{
			name: "invalid YAML",
			setupFile: func(t *testing.T) string {
				tmpDir := t.TempDir()
				outputFile := filepath.Join(tmpDir, "invalid.yaml")
				invalidYAML := []byte("invalid: yaml: content: [unclosed")
				if err := os.WriteFile(outputFile, invalidYAML, 0644); err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
				return outputFile
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setupFile(t)
			result, err := ParseOutput(filePath)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.validate != nil && !tt.expectError {
				tt.validate(t, result)
			}
		})
	}
}

func TestFilterRuleSets(t *testing.T) {
	tests := []struct {
		name     string
		input    []konveyor.RuleSet
		expected int
		validate func(t *testing.T, result []konveyor.RuleSet)
	}{
		{
			name: "empty rulesets filtered out",
			input: []konveyor.RuleSet{
				{
					Name:       "empty-ruleset",
					Violations: map[string]konveyor.Violation{},
					Insights:   map[string]konveyor.Violation{},
					Tags:       []string{},
				},
			},
			expected: 0,
		},
		{
			name: "rulesets with violations kept",
			input: []konveyor.RuleSet{
				{
					Name: "ruleset-with-violations",
					Violations: map[string]konveyor.Violation{
						"rule1": {Description: "Test violation"},
					},
				},
				{
					Name:       "empty-ruleset",
					Violations: map[string]konveyor.Violation{},
				},
			},
			expected: 1,
			validate: func(t *testing.T, result []konveyor.RuleSet) {
				if result[0].Name != "ruleset-with-violations" {
					t.Errorf("Expected 'ruleset-with-violations', got '%s'", result[0].Name)
				}
			},
		},
		{
			name: "rulesets with insights kept",
			input: []konveyor.RuleSet{
				{
					Name: "ruleset-with-insights",
					Insights: map[string]konveyor.Violation{
						"insight1": {Description: "Test insight"},
					},
				},
			},
			expected: 1,
		},
		{
			name: "rulesets with tags kept",
			input: []konveyor.RuleSet{
				{
					Name: "ruleset-with-tags",
					Tags: []string{"tag1"},
				},
			},
			expected: 1,
		},
		{
			name: "mixed rulesets",
			input: []konveyor.RuleSet{
				{Name: "empty1"},
				{Name: "with-tags", Tags: []string{"tag1"}},
				{Name: "empty2"},
				{Name: "with-violations", Violations: map[string]konveyor.Violation{"r1": {}}},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterRuleSets(tt.input)
			if len(result) != tt.expected {
				t.Errorf("Expected %d filtered rulesets, got %d", tt.expected, len(result))
			}
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestNormalizeRuleSets(t *testing.T) {
	tests := []struct {
		name        string
		input       []konveyor.RuleSet
		testDir     string
		expectError bool
		validate    func(t *testing.T, result []konveyor.RuleSet)
	}{
		{
			name: "basic normalization",
			input: []konveyor.RuleSet{
				{
					Name:        "test-ruleset",
					Description: "Test Description",
					Tags:        []string{"tag1"},
					Violations: map[string]konveyor.Violation{
						"rule1": {
							Description: "Test violation",
							Incidents: []konveyor.Incident{
								{
									URI:        uri.URI("file:///test/file.go"),
									Message:    "Test message",
									LineNumber: intPtr(10),
								},
							},
						},
					},
				},
			},
			testDir:     "/test",
			expectError: false,
			validate: func(t *testing.T, result []konveyor.RuleSet) {
				if len(result) != 1 {
					t.Errorf("Expected 1 normalized ruleset, got %d", len(result))
				}
				if result[0].Name != "test-ruleset" {
					t.Errorf("Expected ruleset name 'test-ruleset', got '%s'", result[0].Name)
				}
			},
		},
		{
			name: "with insights",
			input: []konveyor.RuleSet{
				{
					Name: "test-ruleset",
					Insights: map[string]konveyor.Violation{
						"insight1": {
							Description: "Test insight",
							Incidents: []konveyor.Incident{
								{
									URI:     uri.URI("file:///test/file.go"),
									Message: "Test message",
								},
							},
						},
					},
				},
			},
			testDir:     "/test",
			expectError: false,
			validate: func(t *testing.T, result []konveyor.RuleSet) {
				if len(result[0].Insights) != 1 {
					t.Errorf("Expected 1 insight, got %d", len(result[0].Insights))
				}
			},
		},
		{
			name: "preserves all fields",
			input: []konveyor.RuleSet{
				{
					Name:        "test-ruleset",
					Description: "Test Description",
					Tags:        []string{"tag1", "tag2"},
					Errors:      map[string]string{"error1": "error message"},
					Unmatched:   []string{"unmatched1"},
					Skipped:     []string{"skipped1"},
					Violations: map[string]konveyor.Violation{
						"rule1": {
							Description: "Test violation",
							Category:    categoryPtr("mandatory"),
							Labels:      []string{"label1"},
							Effort:      intPtr(5),
							Links: []konveyor.Link{
								{URL: "http://example.com", Title: "Example"},
							},
							Incidents: []konveyor.Incident{
								{
									URI:     uri.URI("file:///test/file.go"),
									Message: "Test message",
								},
							},
						},
					},
				},
			},
			testDir:     "/test",
			expectError: false,
			validate: func(t *testing.T, result []konveyor.RuleSet) {
				rs := result[0]
				if rs.Name != "test-ruleset" {
					t.Errorf("Expected Name 'test-ruleset', got '%s'", rs.Name)
				}
				if rs.Description != "Test Description" {
					t.Errorf("Expected Description 'Test Description', got '%s'", rs.Description)
				}
				if len(rs.Tags) != 2 {
					t.Errorf("Expected 2 tags, got %d", len(rs.Tags))
				}
				if len(rs.Errors) != 1 {
					t.Errorf("Expected 1 error, got %d", len(rs.Errors))
				}
				if len(rs.Unmatched) != 1 {
					t.Errorf("Expected 1 unmatched, got %d", len(rs.Unmatched))
				}
				if len(rs.Skipped) != 1 {
					t.Errorf("Expected 1 skipped, got %d", len(rs.Skipped))
				}

				violation := rs.Violations["rule1"]
				if violation.Description != "Test violation" {
					t.Errorf("Expected violation description 'Test violation', got '%s'", violation.Description)
				}
				if len(violation.Labels) != 1 {
					t.Errorf("Expected 1 label, got %d", len(violation.Labels))
				}
				if violation.Effort == nil || *violation.Effort != 5 {
					t.Errorf("Expected effort 5, got %v", violation.Effort)
				}
				if len(violation.Links) != 1 {
					t.Errorf("Expected 1 link, got %d", len(violation.Links))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeRuleSets(tt.input, tt.testDir)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestNormalizeIncident(t *testing.T) {
	tests := []struct {
		name        string
		incident    konveyor.Incident
		testDir     string
		expectedURI uri.URI
		expectError bool
	}{
		{
			name: "no URI",
			incident: konveyor.Incident{
				Message: "Test message",
			},
			testDir:     "/test",
			expectedURI: "",
			expectError: false,
		},
		{
			name: "non-file URI",
			incident: konveyor.Incident{
				URI:     uri.URI("http://example.com"),
				Message: "Test message",
			},
			testDir:     "/test",
			expectedURI: uri.URI("http://example.com"),
			expectError: false,
		},
		{
			name: "basic file URI with test dir",
			incident: konveyor.Incident{
				URI:        uri.URI("file:///test/dir/file.go"),
				Message:    "Test message",
				LineNumber: intPtr(10),
			},
			testDir:     "/test/dir",
			expectedURI: uri.URI("file:///file.go"),
			expectError: false,
		},
		{
			name: "maven repository - root",
			incident: konveyor.Incident{
				URI:     uri.URI("file:///root/.m2/repository/org/example/lib.jar"),
				Message: "Test message",
			},
			testDir:     "",
			expectedURI: uri.URI("file:///m2/org/example/lib.jar"),
			expectError: false,
		},
		{
			name: "maven repository - cache",
			incident: konveyor.Incident{
				URI:     uri.URI("file:///cache/m2/org/example/lib.jar"),
				Message: "Test message",
			},
			testDir:     "",
			expectedURI: uri.URI("file:///m2/org/example/lib.jar"),
			expectError: false,
		},
		{
			name: "maven repository - addon",
			incident: konveyor.Incident{
				URI:     uri.URI("file:///addon/.m2/repository/org/example/lib.jar"),
				Message: "Test message",
			},
			testDir:     "",
			expectedURI: uri.URI("file:///m2/org/example/lib.jar"),
			expectError: false,
		},
		{
			name: "tackle-hub shared source",
			incident: konveyor.Incident{
				URI:     uri.URI("file:///shared/source/app/main.go"),
				Message: "Test message",
			},
			testDir:     "",
			expectedURI: uri.URI("file:///source/app/main.go"),
			expectError: false,
		},
		{
			name: "tackle-hub opt input",
			incident: konveyor.Incident{
				URI:     uri.URI("file:///opt/input/source/app/main.go"),
				Message: "Test message",
			},
			testDir:     "",
			expectedURI: uri.URI("file:///source/app/main.go"),
			expectError: false,
		},
		{
			name: "java-bin temporary directory",
			incident: konveyor.Incident{
				URI:     uri.URI("file:///tmp/java-bin-12345/com/example/Class.class"),
				Message: "Test message",
			},
			testDir:     "",
			expectedURI: uri.URI("file:///source/com/example/Class.class"),
			expectError: false,
		},
		{
			name: "java-bin macOS temporary directory",
			incident: konveyor.Incident{
				URI:     uri.URI("file:///var/folders/xy/abc123/T/java-bin-67890/com/example/Class.class"),
				Message: "Test message",
			},
			testDir:     "",
			expectedURI: uri.URI("file:///source/com/example/Class.class"),
			expectError: false,
		},
		{
			name: "multiple path replacements",
			incident: konveyor.Incident{
				URI:     uri.URI("file:///test/root/.m2/repository/org/example/lib.jar"),
				Message: "Test message",
			},
			testDir:     "/test",
			expectedURI: uri.URI("file:///m2/org/example/lib.jar"),
			expectError: false,
		},
		{
			name: "preserves all incident fields",
			incident: konveyor.Incident{
				URI:        uri.URI("file:///test/file.go"),
				Message:    "Test message",
				CodeSnip:   "code snippet",
				LineNumber: intPtr(42),
			},
			testDir:     "/test",
			expectedURI: uri.URI("file:///file.go"),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizeIncident(tt.incident, tt.testDir)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectedURI != "" && result.URI != tt.expectedURI {
				t.Errorf("Expected URI '%s', got '%s'", tt.expectedURI, result.URI)
			}

			// Validate that other fields are preserved
			if result.Message != tt.incident.Message {
				t.Errorf("Expected message '%s', got '%s'", tt.incident.Message, result.Message)
			}
			if tt.incident.CodeSnip != "" && result.CodeSnip != tt.incident.CodeSnip {
				t.Errorf("Expected CodeSnip '%s', got '%s'", tt.incident.CodeSnip, result.CodeSnip)
			}
			if tt.incident.LineNumber != nil {
				if result.LineNumber == nil || *result.LineNumber != *tt.incident.LineNumber {
					t.Errorf("Expected LineNumber %d, got %v", *tt.incident.LineNumber, result.LineNumber)
				}
			}
		})
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
