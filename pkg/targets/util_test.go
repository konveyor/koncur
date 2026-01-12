package targets

import (
	"testing"
)

func TestIsBinaryFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "JAR file lowercase",
			path:     "app.jar",
			expected: true,
		},
		{
			name:     "WAR file lowercase",
			path:     "app.war",
			expected: true,
		},
		{
			name:     "EAR file lowercase",
			path:     "app.ear",
			expected: true,
		},
		{
			name:     "JAR file uppercase",
			path:     "APP.JAR",
			expected: true,
		},
		{
			name:     "WAR file uppercase",
			path:     "APP.WAR",
			expected: true,
		},
		{
			name:     "EAR file uppercase",
			path:     "APP.EAR",
			expected: true,
		},
		{
			name:     "JAR with path",
			path:     "path/to/app.jar",
			expected: true,
		},
		{
			name:     "WAR with absolute path",
			path:     "/absolute/path/app.war",
			expected: true,
		},
		{
			name:     "EAR with complex path",
			path:     "/Users/test/projects/myapp/target/myapp-1.0.0.ear",
			expected: true,
		},
		{
			name:     "Java source file",
			path:     "app.java",
			expected: false,
		},
		{
			name:     "Tar archive",
			path:     "app.tar",
			expected: false,
		},
		{
			name:     "Git URL",
			path:     "https://github.com/user/repo.git",
			expected: false,
		},
		{
			name:     "Directory path",
			path:     "/path/to/source",
			expected: false,
		},
		{
			name:     "XML file",
			path:     "pom.xml",
			expected: false,
		},
		{
			name:     "No extension",
			path:     "myapp",
			expected: false,
		},
		{
			name:     "ZIP file",
			path:     "archive.zip",
			expected: false,
		},
		{
			name:     "Mixed case WAR",
			path:     "MyApp.War",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBinaryFile(tt.path)
			if result != tt.expected {
				t.Errorf("IsBinaryFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGitURL(tt.str); got != tt.want {
				t.Errorf("IsGitURL(%q) = %v, want %v", tt.str, got, tt.want)
			}
		})
	}
}
