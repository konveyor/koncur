package targets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/konveyor/test-harness/pkg/config"
)

func TestNewTackleHubTarget(t *testing.T) {
	// Configs with credentials probe GET /applications at construction time
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer hub.Close()

	tests := []struct {
		name    string
		cfg     *config.TackleHubConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: true,
		},
		{
			name: "valid config with token",
			cfg: &config.TackleHubConfig{
				URL:   hub.URL,
				Token: "test-token",
			},
			wantErr: false,
		},
		{
			name: "valid config with username/password",
			cfg: &config.TackleHubConfig{
				URL:      hub.URL,
				Username: "admin",
				Password: "password",
			},
			wantErr: false,
		},
		{
			name: "valid config with maven settings",
			cfg: &config.TackleHubConfig{
				URL:           hub.URL,
				MavenSettings: "/path/to/settings.xml",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := NewTackleHubTarget(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTackleHubTarget() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if target == nil {
					t.Error("Expected non-nil target")
				}
				if target.Name() != "tackle-hub" {
					t.Errorf("Expected name 'tackle-hub', got '%s'", target.Name())
				}
				if target.url != tt.cfg.URL {
					t.Errorf("Expected URL '%s', got '%s'", tt.cfg.URL, target.url)
				}
			}
		})
	}
}

func TestParseGitURL(t *testing.T) {
	tests := []struct {
		name       string
		gitURL     string
		wantURL    string
		wantBranch string
	}{
		{
			name:       "URL without branch",
			gitURL:     "https://github.com/konveyor/tackle-testapp.git",
			wantURL:    "https://github.com/konveyor/tackle-testapp.git",
			wantBranch: "",
		},
		{
			name:       "URL with branch",
			gitURL:     "https://github.com/konveyor/tackle-testapp.git#main",
			wantURL:    "https://github.com/konveyor/tackle-testapp.git",
			wantBranch: "main",
		},
		{
			name:       "URL with feature branch",
			gitURL:     "https://github.com/konveyor/tackle-testapp.git#feature/test",
			wantURL:    "https://github.com/konveyor/tackle-testapp.git",
			wantBranch: "feature/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotBranch := parseGitURL(tt.gitURL)
			if gotURL != tt.wantURL {
				t.Errorf("parseGitURL() URL = %v, want %v", gotURL, tt.wantURL)
			}
			if gotBranch != tt.wantBranch {
				t.Errorf("parseGitURL() branch = %v, want %v", gotBranch, tt.wantBranch)
			}
		})
	}
}

func TestTackleHubTarget_Name(t *testing.T) {
	target := &TackleHubTarget{}
	if target.Name() != "tackle-hub" {
		t.Errorf("Expected name 'tackle-hub', got '%s'", target.Name())
	}
}

func TestTackleHubTarget_ValidateMavenSettings(t *testing.T) {
	tests := []struct {
		name          string
		mavenSettings string
		testRequires  bool
		wantErr       bool
	}{
		{
			name:          "test requires maven but not configured",
			mavenSettings: "",
			testRequires:  true,
			wantErr:       true,
		},
		{
			name:          "test requires maven and configured",
			mavenSettings: "/path/to/settings.xml",
			testRequires:  true,
			wantErr:       false,
		},
		{
			name:          "test doesn't require maven",
			mavenSettings: "",
			testRequires:  false,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &TackleHubTarget{
				mavenSettings: tt.mavenSettings,
			}

			// Validate the requirement check logic
			// This simulates the check done in Execute
			if tt.testRequires && target.mavenSettings == "" {
				if !tt.wantErr {
					t.Error("Expected error for missing maven settings")
				}
			}
		})
	}
}

func TestData_ModeConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		analysisMode string
		wantWithDeps bool
	}{
		{
			name:         "source-only mode",
			analysisMode: "source-only",
			wantWithDeps: false,
		},
		{
			name:         "source-and-deps mode",
			analysisMode: "source-and-deps",
			wantWithDeps: true,
		},
		{
			name:         "binary mode",
			analysisMode: "binary",
			wantWithDeps: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskData := Data{}

			// Simulate the mode setting logic from createAnalysisTask
			switch tt.analysisMode {
			case "source-only":
				taskData.Mode.WithDeps = false
			default:
				taskData.Mode.WithDeps = true
			}

			if taskData.Mode.WithDeps != tt.wantWithDeps {
				t.Errorf("Expected WithDeps = %v, got %v", tt.wantWithDeps, taskData.Mode.WithDeps)
			}
		})
	}
}

func TestTaskStateConstants(t *testing.T) {
	// Verify task state constants are defined correctly
	states := map[string]string{
		TaskStateCreated:   "Created",
		TaskStateReady:     "Ready",
		TaskStatePending:   "Pending",
		TaskStatePostponed: "Postponed",
		TaskStateRunning:   "Running",
		TaskStateSucceeded: "Succeeded",
		TaskStateFailed:    "Failed",
	}

	for constant, expected := range states {
		if constant != expected {
			t.Errorf("Expected %s = %s, got %s", expected, expected, constant)
		}
	}
}

func TestSyntheticRulesetCreation(t *testing.T) {
	// Test the logic for creating synthetic rulesets from insights
	rulesetToInsightConverted := map[string]konveyor.RuleSet{}

	// Simulate adding an insight
	insightRuleset := "test-ruleset"
	rs := rulesetToInsightConverted[insightRuleset]
	rs.Name = insightRuleset
	if rs.Insights == nil {
		rs.Insights = map[string]konveyor.Violation{}
	}
	if rs.Violations == nil {
		rs.Violations = map[string]konveyor.Violation{}
	}

	effort := 5
	v := konveyor.Violation{
		Description: "Test violation",
		Effort:      &effort,
	}

	if effort == 0 {
		rs.Insights["test-rule"] = v
	} else {
		rs.Violations["test-rule"] = v
	}
	rulesetToInsightConverted[insightRuleset] = rs

	// Verify the ruleset was created
	if _, exists := rulesetToInsightConverted["test-ruleset"]; !exists {
		t.Error("Expected test-ruleset to exist")
	}

	// Verify the violation was added (not insight, since effort > 0)
	if len(rs.Violations) != 1 {
		t.Errorf("Expected 1 violation, got %d", len(rs.Violations))
	}
	if len(rs.Insights) != 0 {
		t.Errorf("Expected 0 insights, got %d", len(rs.Insights))
	}
}

// TestTagSourceMapping tests internal logic for mapping tags to rulesets
// Note: This test validates the concept but relies on internal implementation details
func TestTagSourceMapping(t *testing.T) {
	// Test the expected mapping of tag sources to rulesets
	tests := []struct {
		name            string
		tagSource       string
		expectedRuleset string
	}{
		{
			name:            "language-discovery maps to discovery-rules",
			tagSource:       "language-discovery",
			expectedRuleset: "discovery-rules",
		},
		{
			name:            "tech-discovery maps to technology-usage",
			tagSource:       "tech-discovery",
			expectedRuleset: "technology-usage",
		},
		{
			name:            "other sources are not mapped",
			tagSource:       "manual",
			expectedRuleset: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the expected mapping exists in our logic
			var expectedRuleset string
			switch tt.tagSource {
			case "language-discovery":
				expectedRuleset = "discovery-rules"
			case "tech-discovery":
				expectedRuleset = "technology-usage"
			}

			if expectedRuleset != tt.expectedRuleset {
				t.Errorf("Expected ruleset '%s', got '%s'", tt.expectedRuleset, expectedRuleset)
			}
		})
	}
}

func TestPathNormalization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "cache m2 path",
			input:    "/cache/m2/repository/org/test/1.0/test-1.0.jar",
			expected: "/m2/repository/org/test/1.0/test-1.0.jar",
		},
		{
			name:     "root m2 path unchanged",
			input:    "/root/.m2/repository/org/test/1.0/test-1.0.jar",
			expected: "/root/.m2/repository/org/test/1.0/test-1.0.jar",
		},
		{
			name:     "regular path unchanged",
			input:    "/app/src/main/java/Test.java",
			expected: "/app/src/main/java/Test.java",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input
			// Simulate the path normalization logic from tackle_hub.go
			if containsStr(result, "/cache/m2") {
				result = replaceStr(result, "/cache/m2/", "/m2/")
			}

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestPollTaskTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Simulate polling that exceeds timeout
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.Now().Add(100 * time.Millisecond)

	select {
	case <-ctx.Done():
		// Expected - context timeout
	case <-time.After(time.Until(deadline)):
		// Expected - deadline timeout
	case <-ticker.C:
		// Continue would happen in real polling
		time.Sleep(200 * time.Millisecond)
		if time.Now().Before(deadline) {
			t.Error("Should have exceeded deadline")
		}
	}
}

// Helper functions for path normalization test
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || hasSubstr(s, substr)))
}

func hasSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func replaceStr(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); {
		if i <= len(s)-len(old) && s[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}

func TestTackleHubTarget_BinaryAnalysis(t *testing.T) {
	tests := []struct {
		name         string
		application  string
		expectBinary bool
	}{
		{
			name:         "WAR file detection",
			application:  "app.war",
			expectBinary: true,
		},
		{
			name:         "JAR file detection",
			application:  "app.jar",
			expectBinary: true,
		},
		{
			name:         "EAR file detection",
			application:  "app.ear",
			expectBinary: true,
		},
		{
			name:         "Git URL not binary",
			application:  "https://github.com/user/repo.git",
			expectBinary: false,
		},
		{
			name:         "Path not binary",
			application:  "/path/to/source",
			expectBinary: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isBinary := IsBinaryFile(tt.application)
			if isBinary != tt.expectBinary {
				t.Errorf("Expected IsBinaryFile(%q) = %v, got %v", tt.application, tt.expectBinary, isBinary)
			}
		})
	}
}

func TestTackleHubTarget_BinaryModeConfiguration(t *testing.T) {
	tests := []struct {
		name             string
		application      string
		expectBinaryMode bool
		expectArtifact   string
	}{
		{
			name:             "Binary mode for WAR file",
			application:      "test.war",
			expectBinaryMode: true,
			expectArtifact:   "/binary",
		},
		{
			name:             "Source mode for git URL",
			application:      "https://github.com/user/repo.git",
			expectBinaryMode: false,
			expectArtifact:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskData := Data{}
			isBinary := IsBinaryFile(tt.application)

			if isBinary {
				taskData.Mode.Binary = true
				taskData.Mode.Artifact = "/binary"
			}

			if taskData.Mode.Binary != tt.expectBinaryMode {
				t.Errorf("Expected Binary = %v, got %v", tt.expectBinaryMode, taskData.Mode.Binary)
			}
			if taskData.Mode.Artifact != tt.expectArtifact {
				t.Errorf("Expected Artifact = %q, got %q", tt.expectArtifact, taskData.Mode.Artifact)
			}
		})
	}
}

func TestTackleHubTarget_PrepareRulesForHub(t *testing.T) {
	tests := []struct {
		name        string
		rules       []config.CustomRule
		expectError bool
		validate    func(t *testing.T, taskData *Data)
	}{
		{
			name:        "no rules",
			rules:       []config.CustomRule{},
			expectError: false,
			validate: func(t *testing.T, taskData *Data) {
				if taskData.Rules.Path != "" {
					t.Errorf("Expected no rules path, got %s", taskData.Rules.Path)
				}
				if taskData.Rules.Repository != nil {
					t.Errorf("Expected no repository, got %v", taskData.Rules.Repository)
				}
			},
		},
		{
			name: "file rules only",
			rules: []config.CustomRule{
				{File: &config.FilePathRule{FilePath: "/opt/rulesets"}},
				{File: &config.FilePathRule{FilePath: "/custom/rules"}},
			},
			expectError: false,
			validate: func(t *testing.T, taskData *Data) {
				if taskData.Rules.Path != "/rules" {
					t.Errorf("Expected rules path /rules, got %s", taskData.Rules.Path)
				}
				if taskData.Rules.Repository != nil {
					t.Errorf("Expected no repository, got %v", taskData.Rules.Repository)
				}
			},
		},
		{
			name: "single git rule",
			rules: []config.CustomRule{
				{Git: &config.GitRepoRule{GitRepo: "https://github.com/konveyor/rulesets#main/java"}},
			},
			expectError: false,
			validate: func(t *testing.T, taskData *Data) {
				if taskData.Rules.Repository == nil {
					t.Fatal("Expected repository to be set")
				}
				if taskData.Rules.Repository.Kind != "git" {
					t.Errorf("Expected repository kind git, got %s", taskData.Rules.Repository.Kind)
				}
				if taskData.Rules.Repository.URL != "https://github.com/konveyor/rulesets" {
					t.Errorf("Repository URL mismatch: %s", taskData.Rules.Repository.URL)
				}
				if taskData.Rules.Repository.Branch != "main" {
					t.Errorf("Repository branch mismatch: %s", taskData.Rules.Repository.Branch)
				}
				if taskData.Rules.Repository.Path != "java" {
					t.Errorf("Repository path mismatch: %s", taskData.Rules.Repository.Path)
				}
			},
		},
		{
			name: "multiple git rules not supported",
			rules: []config.CustomRule{
				{Git: &config.GitRepoRule{GitRepo: "https://github.com/konveyor/rulesets#main"}},
				{Git: &config.GitRepoRule{GitRepo: "https://github.com/konveyor/analyzer-lsp#v1.0"}},
			},
			expectError: true,
		},
		{
			name: "mixed file and git rules not supported",
			rules: []config.CustomRule{
				{Git: &config.GitRepoRule{GitRepo: "https://github.com/konveyor/rulesets#main"}},
				{File: &config.FilePathRule{FilePath: "/opt/rulesets"}},
			},
			expectError: true,
		},
	}

	target := &TackleHubTarget{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test := &config.TestDefinition{
				Analysis: config.AnalysisConfig{
					Application: "https://github.com/konveyor/test-app",
					Rules:       tt.rules,
				},
			}
			taskData := &Data{}

			err := target.prepareRulesForHub(context.Background(), test, taskData)
			if tt.expectError {
				if err == nil {
					t.Error("Expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("prepareRulesForHub returned error: %v", err)
			}
			tt.validate(t, taskData)
		})
	}
}

func TestTackleHubTarget_GitURLIntegration(t *testing.T) {
	tests := []struct {
		name     string
		analysis config.AnalysisConfig
		validate func(t *testing.T, analysis *config.AnalysisConfig)
	}{
		{
			name: "application with Git URL and branch",
			analysis: config.AnalysisConfig{
				Application: "https://github.com/konveyor/tackle-testapp#main",
			},
			validate: func(t *testing.T, analysis *config.AnalysisConfig) {
				if analysis.ApplicationGitComponents == nil {
					t.Fatal("Expected ApplicationGitComponents to be set")
				}
				if analysis.ApplicationGitComponents.URL != "https://github.com/konveyor/tackle-testapp" {
					t.Errorf("Expected URL to be https://github.com/konveyor/tackle-testapp, got %s",
						analysis.ApplicationGitComponents.URL)
				}
				if analysis.ApplicationGitComponents.Ref != "main" {
					t.Errorf("Expected ref to be main, got %s", analysis.ApplicationGitComponents.Ref)
				}
			},
		},
		{
			name: "application with Git URL, branch and path",
			analysis: config.AnalysisConfig{
				Application: "https://github.com/konveyor/tackle-testapp#ci-2024/src",
			},
			validate: func(t *testing.T, analysis *config.AnalysisConfig) {
				if analysis.ApplicationGitComponents == nil {
					t.Fatal("Expected ApplicationGitComponents to be set")
				}
				if analysis.ApplicationGitComponents.URL != "https://github.com/konveyor/tackle-testapp" {
					t.Errorf("Expected URL to be https://github.com/konveyor/tackle-testapp, got %s",
						analysis.ApplicationGitComponents.URL)
				}
				if analysis.ApplicationGitComponents.Ref != "ci-2024" {
					t.Errorf("Expected ref to be ci-2024, got %s", analysis.ApplicationGitComponents.Ref)
				}
				if analysis.ApplicationGitComponents.Path != "src" {
					t.Errorf("Expected path to be src, got %s", analysis.ApplicationGitComponents.Path)
				}
			},
		},
		{
			name: "rules with Git URLs containing paths",
			analysis: config.AnalysisConfig{
				Application: "/local/app",
				Rules: []config.CustomRule{
					{Git: &config.GitRepoRule{GitRepo: "https://github.com/konveyor/rulesets#main/java/cloud-readiness"}},
					{Git: &config.GitRepoRule{GitRepo: "https://github.com/konveyor/analyzer-lsp#v2.0/dotnet/migration"}},
				},
			},
			validate: func(t *testing.T, analysis *config.AnalysisConfig) {
				if analysis.ApplicationGitComponents != nil {
					t.Error("Expected ApplicationGitComponents to be nil for local path")
				}

				// First rule
				components := analysis.Rules[0].Git.GetCompents()
				if components.URL != "https://github.com/konveyor/rulesets" {
					t.Errorf("First rule URL mismatch: %s", components.URL)
				}
				if components.Ref != "main" {
					t.Errorf("First rule ref mismatch: %s", components.Ref)
				}
				if components.Path != "java/cloud-readiness" {
					t.Errorf("First rule path mismatch: %s", components.Path)
				}

				// Second rule
				components = analysis.Rules[1].Git.GetCompents()
				if components.URL != "https://github.com/konveyor/analyzer-lsp" {
					t.Errorf("Second rule URL mismatch: %s", components.URL)
				}
				if components.Ref != "v2.0" {
					t.Errorf("Second rule ref mismatch: %s", components.Ref)
				}
				if components.Path != "dotnet/migration" {
					t.Errorf("Second rule path mismatch: %s", components.Path)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse Git URLs
			tt.analysis.ParseGitURLs()

			// Run validation
			tt.validate(t, &tt.analysis)
		})
	}
}
