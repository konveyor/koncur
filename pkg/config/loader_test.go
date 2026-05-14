package config

import (
	"reflect"
	"testing"
)

func TestParseSkipCommentPreamble(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		yaml        string
		wantAll     bool
		wantTargets []string
		wantFound   bool
	}{
		{
			name:      "no marker",
			yaml:      "name: t\n",
			wantFound: false,
		},
		{
			name:      "legacy reason skips all",
			yaml:      "# SKIPPED: Skip test due to a known issue on Windows.\nname: x\n",
			wantAll:   true,
			wantFound: true,
		},
		{
			name:        "kantra only",
			yaml:        "# SKIPPED: kantra\nname: x\n",
			wantTargets: []string{"kantra"},
			wantFound:   true,
		},
		{
			name:        "tackle-hub token",
			yaml:        "# SKIPPED: tackle-hub\nname: x\n",
			wantTargets: []string{"tackle-hub"},
			wantFound:   true,
		},
		{
			name:        "hub alias",
			yaml:        "# SKIPPED: hub\nname: x\n",
			wantTargets: []string{"tackle-hub"},
			wantFound:   true,
		},
		{
			name:        "comma union",
			yaml:        "# SKIPPED: kantra, tackle-hub\nname: x\n",
			wantTargets: []string{"kantra", "tackle-hub"},
			wantFound:   true,
		},
		{
			name:      "bare marker",
			yaml:      "# SKIPPED\nname: x\n",
			wantAll:   true,
			wantFound: true,
		},
		{
			name:        "case insensitive keyword",
			yaml:        "# skipped: kantra\nname: x\n",
			wantFound:   true,
			wantTargets: []string{"kantra"},
		},
		{
			name:      "skipped comment after yaml starts is ignored",
			yaml:      "name: x\n# SKIPPED: kantra\n",
			wantFound: false,
		},
		{
			name:        "document marker then skipped comment",
			yaml:        "---\n# SKIPPED: kantra\nname: x\n",
			wantFound:   true,
			wantTargets: []string{"kantra"},
		},
		{
			name:        "yaml directive and marker then skipped",
			yaml:        "%YAML 1.1\n---\n# SKIPPED: hub\nname: x\n",
			wantFound:   true,
			wantTargets: []string{"tackle-hub"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotAll, gotTargets, gotFound := parseSkipCommentPreamble([]byte(tt.yaml))
			if gotFound != tt.wantFound {
				t.Fatalf("found: got %v want %v", gotFound, tt.wantFound)
			}
			if gotAll != tt.wantAll {
				t.Fatalf("skipAll: got %v want %v", gotAll, tt.wantAll)
			}
			if !reflect.DeepEqual(gotTargets, tt.wantTargets) {
				t.Fatalf("targets: got %#v want %#v", gotTargets, tt.wantTargets)
			}
		})
	}
}

func TestTestDefinitionShouldSkipForTarget(t *testing.T) {
	t.Parallel()
	td := &TestDefinition{}
	td.commentSkipAll = false
	td.commentSkipTargets = []string{"kantra"}
	if !td.ShouldSkipForTarget("kantra") {
		t.Fatal("expected skip for kantra")
	}
	if td.ShouldSkipForTarget("tackle-hub") {
		t.Fatal("did not expect skip for tackle-hub")
	}
}
