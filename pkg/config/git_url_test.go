package config

import (
	"testing"
)

func TestParseGitURLWithPath(t *testing.T) {
	tests := []struct {
		name     string
		gitURL   string
		wantURL  string
		wantRef  string
		wantPath string
	}{
		{
			name:     "URL with branch and path",
			gitURL:   "https://github.com/konveyor/rules.git#main/rulesets/java",
			wantURL:  "https://github.com/konveyor/rules.git",
			wantRef:  "main",
			wantPath: "rulesets/java",
		},
		{
			name:     "URL with branch only",
			gitURL:   "https://github.com/konveyor/rules.git#main",
			wantURL:  "https://github.com/konveyor/rules.git",
			wantRef:  "main",
			wantPath: "",
		},
		{
			name:     "URL without branch or path",
			gitURL:   "https://github.com/konveyor/rules.git",
			wantURL:  "https://github.com/konveyor/rules.git",
			wantRef:  "",
			wantPath: "",
		},
		{
			name:     "URL with deep path",
			gitURL:   "https://github.com/konveyor/analyzer.git#v1.0.0/rulesets/technology/java",
			wantURL:  "https://github.com/konveyor/analyzer.git",
			wantRef:  "v1.0.0",
			wantPath: "rulesets/technology/java",
		},
		{
			name:     "URL with feature branch and path",
			gitURL:   "https://github.com/konveyor/rules.git#feature/new-rules/custom",
			wantURL:  "https://github.com/konveyor/rules.git",
			wantRef:  "feature",
			wantPath: "new-rules/custom",
		},
		{
			name:     "C# analyzer provider example",
			gitURL:   "https://github.com/konveyor/c-sharp-analyzer-provider#main/rulesets/dotnet-core-migration",
			wantURL:  "https://github.com/konveyor/c-sharp-analyzer-provider",
			wantRef:  "main",
			wantPath: "rulesets/dotnet-core-migration",
		},
		{
			name:     "Application example with subpath",
			gitURL:   "https://github.com/konveyor/tackle-testapp-public#ci-2024",
			wantURL:  "https://github.com/konveyor/tackle-testapp-public",
			wantRef:  "ci-2024",
			wantPath: "",
		},
		{
			name:     "Book server with branch",
			gitURL:   "https://github.com/konveyor-ecosystem/book-server#ci-oct2025",
			wantURL:  "https://github.com/konveyor-ecosystem/book-server",
			wantRef:  "ci-oct2025",
			wantPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			components := ParseGitURLWithPath(tt.gitURL)
			if components.URL != tt.wantURL {
				t.Errorf("ParseGitURLWithPath() URL = %v, want %v", components.URL, tt.wantURL)
			}
			if components.Ref != tt.wantRef {
				t.Errorf("ParseGitURLWithPath() Ref = %v, want %v", components.Ref, tt.wantRef)
			}
			if components.Path != tt.wantPath {
				t.Errorf("ParseGitURLWithPath() Path = %v, want %v", components.Path, tt.wantPath)
			}
		})
	}
}

func TestIsGitURL(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want bool
	}{
		{"HTTPS URL", "https://github.com/konveyor/rules.git", true},
		{"HTTP URL", "http://github.com/konveyor/rules.git", true},
		{"Git SSH URL", "git@github.com:konveyor/rules.git", true},
		{"URL with reference", "https://github.com/konveyor/rules#main", true},
		{"Local path with hash", "/path/to/rules#something", true}, // Contains #
		{"Local path", "/path/to/rules", false},
		{"Relative path", "./rules", false},
		{"File URL", "file:///path/to/rules", false},
		{"Rules directory", "/opt/rulesets", false},
		{"Current directory", ".", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGitURL(tt.str); got != tt.want {
				t.Errorf("IsGitURL(%q) = %v, want %v", tt.str, got, tt.want)
			}
		})
	}
}

func TestAnalysisConfig_ParseGitURLs(t *testing.T) {
	tests := []struct {
		name        string
		config      AnalysisConfig
		wantAppComp *GitURLComponents
		wantRules   int
	}{
		{
			name: "Git application and mixed rules",
			config: AnalysisConfig{
				Application: "https://github.com/konveyor/app#main/src",
				Rules: []string{
					"https://github.com/konveyor/rules#v1.0/java",
					"/local/rules",
					"https://github.com/konveyor/rules2#main",
				},
			},
			wantAppComp: &GitURLComponents{
				URL:  "https://github.com/konveyor/app",
				Ref:  "main",
				Path: "src",
			},
			wantRules: 3,
		},
		{
			name: "Local application with Git rules",
			config: AnalysisConfig{
				Application: "/local/app",
				Rules: []string{
					"https://github.com/konveyor/rules#main/rulesets",
				},
			},
			wantAppComp: nil,
			wantRules:   1,
		},
		{
			name: "No rules",
			config: AnalysisConfig{
				Application: "https://github.com/konveyor/app#main",
				Rules:       []string{},
			},
			wantAppComp: &GitURLComponents{
				URL: "https://github.com/konveyor/app",
				Ref: "main",
			},
			wantRules: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := tt.config
			ac.ParseGitURLs()

			// Check application components
			if tt.wantAppComp != nil {
				if ac.ApplicationGitComponents == nil {
					t.Error("Expected ApplicationGitComponents to be set")
				} else {
					if ac.ApplicationGitComponents.URL != tt.wantAppComp.URL {
						t.Errorf("ApplicationGitComponents.URL = %v, want %v",
							ac.ApplicationGitComponents.URL, tt.wantAppComp.URL)
					}
					if ac.ApplicationGitComponents.Ref != tt.wantAppComp.Ref {
						t.Errorf("ApplicationGitComponents.Ref = %v, want %v",
							ac.ApplicationGitComponents.Ref, tt.wantAppComp.Ref)
					}
					if ac.ApplicationGitComponents.Path != tt.wantAppComp.Path {
						t.Errorf("ApplicationGitComponents.Path = %v, want %v",
							ac.ApplicationGitComponents.Path, tt.wantAppComp.Path)
					}
				}
			} else {
				if ac.ApplicationGitComponents != nil {
					t.Error("Expected ApplicationGitComponents to be nil")
				}
			}

			// Check rules components
			if len(ac.RulesGitComponents) != tt.wantRules {
				t.Errorf("RulesGitComponents length = %v, want %v",
					len(ac.RulesGitComponents), tt.wantRules)
			}
		})
	}
}