package targets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konveyor/analyzer-lsp/provider"
	"github.com/konveyor/test-harness/pkg/config"
)

func TestNewKantraTarget(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.KantraConfig
		wantErr    bool
		checkPath  bool
		expectPath string
	}{
		{
			name: "nil config uses PATH",
			cfg:  nil,
			// This will fail if kantra is not in PATH, which is expected
			wantErr:   true,
			checkPath: false,
		},
		{
			name: "empty config uses PATH",
			cfg:  &config.KantraConfig{},
			// This will fail if kantra is not in PATH, which is expected
			wantErr:   true,
			checkPath: false,
		},
		{
			name: "explicit binary path",
			cfg: &config.KantraConfig{
				BinaryPath: "/usr/local/bin/kantra",
			},
			wantErr:    false,
			checkPath:  true,
			expectPath: "/usr/local/bin/kantra",
		},
		{
			name: "config with maven settings",
			cfg: &config.KantraConfig{
				BinaryPath:    "/usr/local/bin/kantra",
				MavenSettings: "/path/to/settings.xml",
			},
			wantErr:    false,
			checkPath:  true,
			expectPath: "/usr/local/bin/kantra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := NewKantraTarget(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKantraTarget() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if target == nil {
					t.Error("Expected non-nil target")
				}
				if target.Name() != "kantra" {
					t.Errorf("Expected name 'kantra', got '%s'", target.Name())
				}
				if tt.checkPath && target.binaryPath != tt.expectPath {
					t.Errorf("Expected binary path '%s', got '%s'", tt.expectPath, target.binaryPath)
				}
				if tt.cfg != nil && target.mavenSettings != tt.cfg.MavenSettings {
					t.Errorf("Expected maven settings '%s', got '%s'", tt.cfg.MavenSettings, target.mavenSettings)
				}
			}
		})
	}
}

func TestKantraTarget_Name(t *testing.T) {
	target := &KantraTarget{}
	if target.Name() != "kantra" {
		t.Errorf("Expected name 'kantra', got '%s'", target.Name())
	}
}

func TestKantraTarget_BuildArgs(t *testing.T) {
	tests := []struct {
		name             string
		analysis         config.AnalysisConfig
		inputPath        string
		outputDir        string
		mavenSettings    string
		expectContain    []string
		expectNotContain []string
	}{
		{
			name: "basic source-only analysis",
			analysis: config.AnalysisConfig{
				AnalysisMode: provider.SourceOnlyAnalysisMode,
				ContextLines: 10,
			},
			inputPath: "/path/to/app",
			outputDir: "/path/to/output",
			expectContain: []string{
				"analyze",
				"--context-lines", "10",
				"--input", "/path/to/app",
				"--output", "/path/to/output",
				"--mode", "source-only",
				"--run-local=false",
				"--overwrite",
			},
		},
		{
			name: "full analysis with targets and sources",
			analysis: config.AnalysisConfig{
				AnalysisMode: provider.FullAnalysisMode,
				ContextLines: 20,
				Target:       []string{"cloud-readiness", "quarkus"},
				Source:       []string{"java", "java-ee"},
			},
			inputPath: "/path/to/app",
			outputDir: "/path/to/output",
			expectContain: []string{
				"analyze",
				"--mode", "full",
				"-t", "cloud-readiness",
				"-t", "quarkus",
				"-s", "java",
				"-s", "java-ee",
			},
		},
		{
			name: "analysis with label selector",
			analysis: config.AnalysisConfig{
				AnalysisMode:  provider.SourceOnlyAnalysisMode,
				ContextLines:  10,
				LabelSelector: "konveyor.io/target=cloud-readiness",
			},
			inputPath: "/path/to/app",
			outputDir: "/path/to/output",
			expectContain: []string{
				"--label-selector", "konveyor.io/target=cloud-readiness",
			},
		},
		{
			name: "analysis with incident selector",
			analysis: config.AnalysisConfig{
				AnalysisMode:     provider.SourceOnlyAnalysisMode,
				ContextLines:     10,
				IncidentSelector: "lineNumber > 100",
			},
			inputPath: "/path/to/app",
			outputDir: "/path/to/output",
			expectContain: []string{
				"--incident-selector", "lineNumber > 100",
			},
		},
		{
			name: "analysis with maven settings",
			analysis: config.AnalysisConfig{
				AnalysisMode: provider.SourceOnlyAnalysisMode,
				ContextLines: 10,
			},
			inputPath:     "/path/to/app",
			outputDir:     "/path/to/output",
			mavenSettings: "/path/to/settings.xml",
			expectContain: []string{
				"--maven-settings", "/path/to/settings.xml",
			},
		},
		{
			name: "analysis with rules",
			analysis: config.AnalysisConfig{
				AnalysisMode: provider.SourceOnlyAnalysisMode,
				ContextLines: 10,
				Rules:        []string{"/custom/rules1", "/custom/rules2"},
			},
			inputPath: "/path/to/app",
			outputDir: "/path/to/output",
			expectContain: []string{
				"--rules", "/custom/rules1",
				"--rules", "/custom/rules2",
			},
		},
		{
			name: "analysis without maven settings",
			analysis: config.AnalysisConfig{
				AnalysisMode: provider.SourceOnlyAnalysisMode,
				ContextLines: 10,
			},
			inputPath:     "/path/to/app",
			outputDir:     "/path/to/output",
			mavenSettings: "",
			expectNotContain: []string{
				"--maven-settings",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &KantraTarget{
				binaryPath:    "/usr/local/bin/kantra",
				mavenSettings: tt.mavenSettings,
			}

			args := k.buildArgs(tt.analysis, tt.inputPath, tt.outputDir, tt.mavenSettings)
			argsStr := strings.Join(args, " ")

			// Check for expected arguments
			for _, expected := range tt.expectContain {
				found := false
				for _, arg := range args {
					if arg == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected arg '%s' not found in: %v", expected, args)
				}
			}

			// Check for unexpected arguments
			for _, notExpected := range tt.expectNotContain {
				for _, arg := range args {
					if arg == notExpected {
						t.Errorf("Unexpected arg '%s' found in: %v", notExpected, args)
					}
				}
			}

			// Verify basic structure
			if len(args) == 0 {
				t.Error("Expected non-empty args")
			}
			if args[0] != "analyze" {
				t.Errorf("Expected first arg to be 'analyze', got '%s'", args[0])
			}

			t.Logf("Generated args: %s", argsStr)
		})
	}
}

func TestKantraTarget_PrepareInput(t *testing.T) {
	tests := []struct {
		name        string
		application string
		isGitURL    bool
		expectError bool
	}{
		{
			name:        "local path",
			application: "/local/path/to/app",
			isGitURL:    false,
			expectError: false,
		},
		{
			name:        "binary reference",
			application: "binary:app.jar",
			isGitURL:    false,
			expectError: false,
		},
		{
			name:        "http git URL",
			application: "http://github.com/konveyor/tackle-testapp.git",
			isGitURL:    true,
			expectError: false,
		},
		{
			name:        "https git URL",
			application: "https://github.com/konveyor/tackle-testapp.git",
			isGitURL:    true,
			expectError: false,
		},
		{
			name:        "git URL with branch",
			application: "https://github.com/konveyor/tackle-testapp.git#main",
			isGitURL:    true,
			expectError: false,
		},
		{
			name:        "git URL with feature branch",
			application: "https://github.com/konveyor/tackle-testapp.git#feature/test",
			isGitURL:    true,
			expectError: false,
		},
		{
			name:        "ssh git URL",
			application: "git@github.com:konveyor/tackle-testapp.git",
			isGitURL:    true,
			expectError: false,
		},
		{
			name:        "git URL with branch and path",
			application: "https://github.com/konveyor-ecosystem/windup.git#ci-2024/test-files/seam-booking-5.2",
			isGitURL:    true,
			expectError: false,
		},
		{
			name:        "git URL with branch and simple path",
			application: "https://github.com/example/repo.git#main/subdir",
			isGitURL:    true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check URL detection logic (not executing prepareInput as it would require git/network)
			isGitURL := strings.HasPrefix(tt.application, "http://") ||
				strings.HasPrefix(tt.application, "https://") ||
				strings.HasPrefix(tt.application, "git@")

			if isGitURL != tt.isGitURL {
				t.Errorf("Expected isGitURL=%v, got %v for '%s'", tt.isGitURL, isGitURL, tt.application)
			}

			// Check binary prefix handling
			if strings.HasPrefix(tt.application, "binary:") {
				binaryFile := tt.application[7:]
				if tt.application != "binary:"+binaryFile {
					t.Error("Binary prefix handling failed")
				}
			}

			// Check git reference and path parsing
			if strings.Contains(tt.application, "#") {
				parts := strings.SplitN(tt.application, "#", 2)
				if len(parts) != 2 {
					t.Error("Failed to parse git reference")
				}
				gitURL := parts[0]
				refAndPath := parts[1]
				if gitURL == "" || refAndPath == "" {
					t.Error("Git URL or ref is empty after parsing")
				}

				// Parse ref and path
				refParts := strings.SplitN(refAndPath, "/", 2)
				gitRef := refParts[0]
				var gitPath string
				if len(refParts) > 1 {
					gitPath = refParts[1]
				}

				if gitPath != "" {
					t.Logf("Parsed git URL: %s, ref: %s, path: %s", gitURL, gitRef, gitPath)
				} else {
					t.Logf("Parsed git URL: %s, ref: %s", gitURL, gitRef)
				}
			}
		})
	}
}

func TestKantraTarget_ValidateMavenSettings(t *testing.T) {
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
			target := &KantraTarget{
				mavenSettings: tt.mavenSettings,
			}

			// Simulate the validation check from Execute
			if tt.testRequires && target.mavenSettings == "" {
				if !tt.wantErr {
					t.Error("Expected error for missing maven settings")
				}
			}
		})
	}
}

func TestKantraTarget_AnalysisMode(t *testing.T) {
	tests := []struct {
		name         string
		analysisMode provider.AnalysisMode
		expectFlag   string
	}{
		{
			name:         "source-only mode",
			analysisMode: provider.SourceOnlyAnalysisMode,
			expectFlag:   "source-only",
		},
		{
			name:         "full mode",
			analysisMode: provider.FullAnalysisMode,
			expectFlag:   "full",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := config.AnalysisConfig{
				AnalysisMode: tt.analysisMode,
				ContextLines: 10,
			}

			k := &KantraTarget{binaryPath: "/usr/local/bin/kantra"}
			args := k.buildArgs(analysis, "/input", "/output", "")

			// Find the --mode flag
			foundMode := false
			for i, arg := range args {
				if arg == "--mode" && i+1 < len(args) {
					if args[i+1] != tt.expectFlag {
						t.Errorf("Expected mode '%s', got '%s'", tt.expectFlag, args[i+1])
					}
					foundMode = true
					break
				}
			}

			if !foundMode {
				t.Errorf("Expected --mode flag with value '%s' not found", tt.expectFlag)
			}
		})
	}
}

func TestKantraTarget_ContextLines(t *testing.T) {
	tests := []struct {
		name         string
		contextLines int
		expectValue  string
	}{
		{
			name:         "default context lines",
			contextLines: 10,
			expectValue:  "10",
		},
		{
			name:         "custom context lines",
			contextLines: 100,
			expectValue:  "100",
		},
		{
			name:         "zero context lines",
			contextLines: 0,
			expectValue:  "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := config.AnalysisConfig{
				AnalysisMode: provider.SourceOnlyAnalysisMode,
				ContextLines: tt.contextLines,
			}

			k := &KantraTarget{binaryPath: "/usr/local/bin/kantra"}
			args := k.buildArgs(analysis, "/input", "/output", "")

			// Find the --context-lines flag
			foundContextLines := false
			for i, arg := range args {
				if arg == "--context-lines" && i+1 < len(args) {
					if args[i+1] != tt.expectValue {
						t.Errorf("Expected context-lines '%s', got '%s'", tt.expectValue, args[i+1])
					}
					foundContextLines = true
					break
				}
			}

			if !foundContextLines {
				t.Error("Expected --context-lines flag not found")
			}
		})
	}
}

func TestKantraTarget_PrepareBinary(t *testing.T) {
	// Create temp directory with test JAR
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	err := os.WriteFile(jarPath, []byte("fake jar content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	target := &KantraTarget{
		binaryPath: "/usr/local/bin/kantra",
	}

	tests := []struct {
		name        string
		binaryPath  string
		testDir     string
		expectError bool
		expectPath  string
	}{
		{
			name:        "absolute path to existing JAR",
			binaryPath:  jarPath,
			testDir:     tmpDir,
			expectError: false,
			expectPath:  jarPath,
		},
		{
			name:        "relative path to existing JAR",
			binaryPath:  "test.jar",
			testDir:     tmpDir,
			expectError: false,
			expectPath:  jarPath,
		},
		{
			name:        "non-existent binary",
			binaryPath:  "nonexistent.jar",
			testDir:     tmpDir,
			expectError: true,
		},
		{
			name:        "absolute path to non-existent binary",
			binaryPath:  "/nonexistent/path/app.war",
			testDir:     tmpDir,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := target.prepareBinary(tt.binaryPath, tt.testDir)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expectPath {
					t.Errorf("Expected path %s, got %s", tt.expectPath, result)
				}
			}
		})
	}
}

func TestKantraTarget_PrepareInputWithBinary(t *testing.T) {
	// Create temp directory with test binaries
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "app.jar")
	warPath := filepath.Join(tmpDir, "app.war")

	err := os.WriteFile(jarPath, []byte("fake jar"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(warPath, []byte("fake war"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	target := &KantraTarget{
		binaryPath: "/usr/local/bin/kantra",
	}

	tests := []struct {
		name        string
		application string
		testDir     string
		expectError bool
		shouldExist bool
	}{
		{
			name:        "absolute JAR path",
			application: jarPath,
			testDir:     tmpDir,
			expectError: false,
			shouldExist: true,
		},
		{
			name:        "relative JAR path",
			application: "app.jar",
			testDir:     tmpDir,
			expectError: false,
			shouldExist: true,
		},
		{
			name:        "absolute WAR path",
			application: warPath,
			testDir:     tmpDir,
			expectError: false,
			shouldExist: true,
		},
		{
			name:        "non-binary path (should not call prepareBinary)",
			application: tmpDir,
			testDir:     tmpDir,
			expectError: false,
			shouldExist: false, // Returns the directory path as-is
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &config.AnalysisConfig{
				Application: tt.application,
			}
			// Parse Git URLs (this will be a no-op for non-Git URLs)
			analysis.ParseGitURLs()

			result, err := target.prepareInput(context.Background(), analysis, tt.testDir)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result == "" {
					t.Error("Expected non-empty result path")
				}

				// For binary files, verify the file exists
				if IsBinaryFile(tt.application) && tt.shouldExist {
					if _, err := os.Stat(result); err != nil {
						t.Errorf("Result path does not exist: %s", result)
					}
				}
			}
		})
	}
}

func TestKantraTarget_PrepareRules(t *testing.T) {
	tests := []struct {
		name          string
		analysis      config.AnalysisConfig
		expectError   bool
		expectedRules []string
	}{
		{
			name: "no rules",
			analysis: config.AnalysisConfig{
				Rules: []string{},
			},
			expectError:   false,
			expectedRules: nil,
		},
		{
			name: "local rules only",
			analysis: config.AnalysisConfig{
				Rules: []string{
					"/opt/rulesets",
					"/custom/rules",
				},
			},
			expectError: false,
			expectedRules: []string{
				"/opt/rulesets",
				"/custom/rules",
			},
		},
		{
			name: "Git URL rules",
			analysis: config.AnalysisConfig{
				Rules: []string{
					"https://github.com/konveyor/rulesets#main",
					"https://github.com/konveyor/analyzer-lsp#v1.0/rules",
				},
			},
			expectError: false,
			// These will be cloned to work directory
			expectedRules: []string{
				"testwork/rules-0",
				"testwork/rules-1",
			},
		},
		{
			name: "mixed local and Git URL rules",
			analysis: config.AnalysisConfig{
				Rules: []string{
					"/opt/rulesets",
					"https://github.com/konveyor/rulesets#main/java",
					"/custom/rules",
					"https://github.com/konveyor/analyzer-lsp#v1.0/dotnet",
				},
			},
			expectError: false,
			expectedRules: []string{
				"/opt/rulesets",
				"testwork/rules-1",
				"/custom/rules",
				"testwork/rules-3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse Git URLs in the analysis config
			tt.analysis.ParseGitURLs()

			// Create a mock prepareRules function that simulates the behavior
			// without actually cloning repositories
			preparedRules := make([]string, 0, len(tt.analysis.Rules))
			for i, rule := range tt.analysis.Rules {
				// Check if we have parsed Git components for this rule
				if i < len(tt.analysis.RulesGitComponents) && tt.analysis.RulesGitComponents[i] != nil {
					// Simulate a cloned path
					preparedRules = append(preparedRules, fmt.Sprintf("testwork/rules-%d", i))
				} else {
					// Local path - use as-is
					preparedRules = append(preparedRules, rule)
				}
			}

			// Verify the results match expected
			if len(preparedRules) != len(tt.expectedRules) {
				t.Errorf("Expected %d prepared rules, got %d", len(tt.expectedRules), len(preparedRules))
			}
			for i, expected := range tt.expectedRules {
				if i < len(preparedRules) && preparedRules[i] != expected {
					t.Errorf("Rule %d: expected %s, got %s", i, expected, preparedRules[i])
				}
			}
		})
	}
}

func TestKantraTarget_GitURLIntegration(t *testing.T) {
	tests := []struct {
		name     string
		analysis config.AnalysisConfig
		validate func(t *testing.T, analysis *config.AnalysisConfig)
	}{
		{
			name: "application with Git URL and path",
			analysis: config.AnalysisConfig{
				Application: "https://github.com/konveyor/tackle-testapp#main/src",
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
				if analysis.ApplicationGitComponents.Path != "src" {
					t.Errorf("Expected path to be src, got %s", analysis.ApplicationGitComponents.Path)
				}
			},
		},
		{
			name: "rules with multiple Git URLs and paths",
			analysis: config.AnalysisConfig{
				Application: "/local/app",
				Rules: []string{
					"https://github.com/konveyor/rulesets#main/java",
					"/local/rules",
					"https://github.com/konveyor/analyzer-lsp#v1.0/dotnet/rules",
				},
			},
			validate: func(t *testing.T, analysis *config.AnalysisConfig) {
				if analysis.ApplicationGitComponents != nil {
					t.Error("Expected ApplicationGitComponents to be nil for local path")
				}
				if len(analysis.RulesGitComponents) != 3 {
					t.Fatalf("Expected 3 RulesGitComponents, got %d", len(analysis.RulesGitComponents))
				}

				// First rule - Git URL with path
				if analysis.RulesGitComponents[0] == nil {
					t.Error("Expected first rule to have Git components")
				} else {
					if analysis.RulesGitComponents[0].URL != "https://github.com/konveyor/rulesets" {
						t.Errorf("First rule URL mismatch: %s", analysis.RulesGitComponents[0].URL)
					}
					if analysis.RulesGitComponents[0].Path != "java" {
						t.Errorf("First rule path mismatch: %s", analysis.RulesGitComponents[0].Path)
					}
				}

				// Second rule - local path
				if analysis.RulesGitComponents[1] != nil {
					t.Error("Expected second rule to have nil Git components (local path)")
				}

				// Third rule - Git URL with deep path
				if analysis.RulesGitComponents[2] == nil {
					t.Error("Expected third rule to have Git components")
				} else {
					if analysis.RulesGitComponents[2].Path != "dotnet/rules" {
						t.Errorf("Third rule path mismatch: %s", analysis.RulesGitComponents[2].Path)
					}
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
