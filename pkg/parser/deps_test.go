package parser

import (
	"path/filepath"
	"testing"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
)

func TestNormalizeDepsFlat_FileURIAndPrefix(t *testing.T) {
	items := []konveyor.DepsFlatItem{
		{
			FileURI:  "file:///opt/input/source/pom.xml",
			Provider: "java",
			Dependencies: []*konveyor.Dep{
				{
					Name:          "acme.lib",
					Version:       "1.0",
					FileURIPrefix: "file:///addon/.m2/repository/acme/lib/1.0",
					Extras: map[string]interface{}{
						"pomPath": "/opt/input/source/pom.xml",
					},
				},
			},
		},
	}
	testDir := filepath.Join("/tmp", "tests", "book-server-deps")
	out, err := NormalizeDepsFlat(items, testDir)
	if err != nil {
		t.Fatalf("NormalizeDepsFlat: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len %d", len(out))
	}
	if got := out[0].FileURI; got != "file:///source/pom.xml" {
		t.Errorf("FileURI = %q", got)
	}
	if len(out[0].Dependencies) != 1 {
		t.Fatalf("deps len")
	}
	p := out[0].Dependencies[0].FileURIPrefix
	if want := "file:///m2/acme/lib/1.0"; p != want {
		t.Errorf("prefix = %q want %q", p, want)
	}
	pom := out[0].Dependencies[0].Extras["pomPath"].(string)
	if pom != "/source/pom.xml" {
		t.Errorf("pomPath = %q", pom)
	}
}
