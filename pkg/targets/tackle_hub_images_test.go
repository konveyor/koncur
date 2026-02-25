package targets

import (
	"encoding/json"
	"testing"

	"github.com/konveyor/test-harness/pkg/config"
)

func TestBuildCRPatch(t *testing.T) {
	tests := []struct {
		name       string
		images     *config.TackleHubImages
		wantEmpty  bool
		wantFields map[string]string
	}{
		{
			name:      "nil images",
			images:    nil,
			wantEmpty: true,
		},
		{
			name:      "empty images",
			images:    &config.TackleHubImages{},
			wantEmpty: true,
		},
		{
			name: "hub image only",
			images: &config.TackleHubImages{
				Hub: "my-hub:dev",
			},
			wantFields: map[string]string{
				"hub_image_fqin": "my-hub:dev",
			},
		},
		{
			name: "analyzer image only",
			images: &config.TackleHubImages{
				Analyzer: "my-analyzer:dev",
			},
			wantFields: map[string]string{
				"analyzer_fqin": "my-analyzer:dev",
			},
		},
		{
			name: "generic provider sets both python and nodejs",
			images: &config.TackleHubImages{
				GenericProvider: "my-generic:dev",
			},
			wantFields: map[string]string{
				"provider_python_image_fqin": "my-generic:dev",
				"provider_nodejs_image_fqin": "my-generic:dev",
			},
		},
		{
			name: "all images",
			images: &config.TackleHubImages{
				Hub:             "my-hub:dev",
				Analyzer:        "my-analyzer:dev",
				JavaProvider:    "my-java:dev",
				GenericProvider: "my-generic:dev",
				CsharpProvider:  "my-csharp:dev",
				Runner:          "my-kantra:dev",
				DiscoveryAddon:  "my-discovery:dev",
				PlatformAddon:   "my-platform:dev",
			},
			wantFields: map[string]string{
				"hub_image_fqin":              "my-hub:dev",
				"analyzer_fqin":               "my-analyzer:dev",
				"provider_java_image_fqin":    "my-java:dev",
				"provider_python_image_fqin":  "my-generic:dev",
				"provider_nodejs_image_fqin":  "my-generic:dev",
				"provider_c_sharp_image_fqin": "my-csharp:dev",
				"kantra_fqin":                 "my-kantra:dev",
				"language_discovery_fqin":     "my-discovery:dev",
				"platform_fqin":               "my-platform:dev",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := buildCRPatch(tt.images)
			if err != nil {
				t.Fatalf("buildCRPatch() error: %v", err)
			}

			if tt.wantEmpty {
				if patch != "" {
					t.Errorf("expected empty patch, got: %s", patch)
				}
				return
			}

			if patch == "" {
				t.Fatal("expected non-empty patch")
			}

			// Parse the patch JSON
			var parsed map[string]map[string]string
			if err := json.Unmarshal([]byte(patch), &parsed); err != nil {
				t.Fatalf("failed to parse patch JSON: %v", err)
			}

			spec, ok := parsed["spec"]
			if !ok {
				t.Fatal("patch missing 'spec' key")
			}

			// Check expected fields
			for key, want := range tt.wantFields {
				got, ok := spec[key]
				if !ok {
					t.Errorf("patch missing field %q", key)
					continue
				}
				if got != want {
					t.Errorf("field %q = %q, want %q", key, got, want)
				}
			}

			// Check no unexpected fields
			for key := range spec {
				if _, ok := tt.wantFields[key]; !ok {
					t.Errorf("unexpected field in patch: %q = %q", key, spec[key])
				}
			}
		})
	}
}

func TestHasImageOverrides(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *config.TackleHubConfig
		expect bool
	}{
		{
			name:   "nil images",
			cfg:    &config.TackleHubConfig{URL: "http://localhost:8080"},
			expect: false,
		},
		{
			name: "empty images struct",
			cfg: &config.TackleHubConfig{
				URL:    "http://localhost:8080",
				Images: &config.TackleHubImages{},
			},
			expect: false,
		},
		{
			name: "hub image set",
			cfg: &config.TackleHubConfig{
				URL:    "http://localhost:8080",
				Images: &config.TackleHubImages{Hub: "my-hub:dev"},
			},
			expect: true,
		},
		{
			name: "only analyzer set",
			cfg: &config.TackleHubConfig{
				URL:    "http://localhost:8080",
				Images: &config.TackleHubImages{Analyzer: "my-analyzer:dev"},
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.HasImageOverrides()
			if got != tt.expect {
				t.Errorf("HasImageOverrides() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestTackleHubConfigDefaults(t *testing.T) {
	cfg := &config.TackleHubConfig{URL: "http://localhost:8080"}

	if ns := cfg.GetNamespace(); ns != "konveyor-tackle" {
		t.Errorf("GetNamespace() = %q, want %q", ns, "konveyor-tackle")
	}
	if cr := cfg.GetCRName(); cr != "tackle" {
		t.Errorf("GetCRName() = %q, want %q", cr, "tackle")
	}

	// Custom values
	cfg.Namespace = "custom-ns"
	cfg.CRName = "custom-cr"

	if ns := cfg.GetNamespace(); ns != "custom-ns" {
		t.Errorf("GetNamespace() = %q, want %q", ns, "custom-ns")
	}
	if cr := cfg.GetCRName(); cr != "custom-cr" {
		t.Errorf("GetCRName() = %q, want %q", cr, "custom-cr")
	}
}
