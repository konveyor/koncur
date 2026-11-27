package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konveyor/test-harness/pkg/config"
)

func TestLeadingCommentBlock(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no comments",
			in:   "name: x\nanalysis: {}\n",
			want: "",
		},
		{
			name: "single comment",
			in:   "# SKIPPED: hub\nname: x\n",
			want: "# SKIPPED: hub\n",
		},
		{
			name: "several comments with a blank line between",
			in:   "# SKIPPED: Windows only issue.\n\n# See https://example.com/issues/1\n\nname: x\n",
			want: "# SKIPPED: Windows only issue.\n\n# See https://example.com/issues/1\n",
		},
		{
			name: "document marker is not copied",
			in:   "---\n# SKIPPED\nname: x\n",
			want: "# SKIPPED\n",
		},
		{
			name: "trailing whitespace and CRLF are trimmed",
			in:   "# SKIPPED: kantra  \r\nname: x\r\n",
			want: "# SKIPPED: kantra\n",
		},
		{
			name: "comments after the first key are left alone",
			in:   "name: x\n# not a header\n",
			want: "",
		},
		{
			name: "empty file",
			in:   "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := leadingCommentBlock([]byte(tt.in)); got != tt.want {
				t.Fatalf("leadingCommentBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}

const generateFixture = `name: sample
description: a sample test
analysis:
  application: https://github.com/example/app
  analysisMode: source-only
forceLocal: true
expect:
  exitCode: 0
  output:
    file: expected-output.yaml
`

func writeFixture(t *testing.T, header string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.yaml")
	if err := os.WriteFile(path, []byte(header+generateFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSaveSimpleTestDefinitionKeepsSkipMarkers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		header  string
		skipFor map[string]bool
	}{
		{
			name:    "skip for every target",
			header:  "# SKIPPED: Skip test due to a known issue on Windows.\n# For more information, see https://example.com/issues/806\n\n",
			skipFor: map[string]bool{"kantra": true, "tackle-hub": true},
		},
		{
			name:    "skip for one target",
			header:  "# SKIPPED: hub\n# Goland provider is not yet supported\n",
			skipFor: map[string]bool{"kantra": false, "tackle-hub": true},
		},
		{
			name:    "no marker stays runnable",
			header:  "",
			skipFor: map[string]bool{"kantra": false, "tackle-hub": false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := writeFixture(t, tt.header)

			test, err := config.LoadWithOptions(path, true)
			if err != nil {
				t.Fatalf("load before save: %v", err)
			}
			test.Expect.ExitCode = 1
			if err := saveSimpleTestDefinition(path, test); err != nil {
				t.Fatalf("save: %v", err)
			}

			reloaded, err := config.LoadWithOptions(path, true)
			if err != nil {
				t.Fatalf("load after save: %v", err)
			}
			for target, want := range tt.skipFor {
				if got := reloaded.ShouldSkipForTarget(target); got != want {
					t.Errorf("ShouldSkipForTarget(%q) = %v after save, want %v", target, got, want)
				}
			}
			if !reloaded.ForceLocal {
				t.Error("forceLocal was dropped by save")
			}
			if reloaded.Expect.ExitCode != 1 {
				t.Errorf("exitCode = %d, want 1", reloaded.Expect.ExitCode)
			}

			saved, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if tt.header != "" && !strings.HasPrefix(string(saved), strings.TrimRight(tt.header, "\n")) {
				t.Errorf("saved file does not start with the original header:\n%s", saved)
			}
		})
	}
}

func TestSaveSimpleTestDefinitionKeepsSkippedField(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.yaml")
	content := "skipped: true\n" + generateFixture
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	test, err := config.LoadWithOptions(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveSimpleTestDefinition(path, test); err != nil {
		t.Fatal(err)
	}
	reloaded, err := config.LoadWithOptions(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.ShouldSkipForTarget("kantra") {
		t.Fatal("skipped: true was dropped by save")
	}
}
