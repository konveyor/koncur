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
