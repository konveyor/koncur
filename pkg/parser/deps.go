package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"go.lsp.dev/uri"
	"gopkg.in/yaml.v3"
)

func ParseDependencies(path string) ([]konveyor.DepsFlatItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read dependencies file %s: %w", path, err)
	}

	var items []konveyor.DepsFlatItem
	if err := yaml.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("failed to parse dependencies YAML: %w", err)
	}

	return items, nil
}

// normalize file URIs, dependency prefixes, and path-like strings in extras
// so actual output can be compared to expected fixtures across environments
func NormalizeDepsFlat(items []konveyor.DepsFlatItem, testDir string) ([]konveyor.DepsFlatItem, error) {
	out := make([]konveyor.DepsFlatItem, 0, len(items))
	for _, item := range items {
		norm := konveyor.DepsFlatItem{
			FileURI:      normalizeDepsFileURI(item.FileURI, testDir),
			Provider:     item.Provider,
			Dependencies: make([]*konveyor.Dep, 0, len(item.Dependencies)),
		}
		for _, d := range item.Dependencies {
			if d == nil {
				continue
			}
			depCopy := *d
			depCopy.FileURIPrefix = normalizeDepsFileURI(depCopy.FileURIPrefix, testDir)
			if depCopy.Extras != nil {
				depCopy.Extras = normalizeDepExtrasMap(depCopy.Extras, testDir)
			}
			norm.Dependencies = append(norm.Dependencies, &depCopy)
		}
		sortDepsStable(norm.Dependencies)
		out = append(out, norm)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FileURI != out[j].FileURI {
			return out[i].FileURI < out[j].FileURI
		}
		return out[i].Provider < out[j].Provider
	})
	return out, nil
}

func normalizeDepsFileURI(s, testDir string) string {
	if s == "" {
		return s
	}
	if !strings.Contains(s, "file://") {
		return NormalizePath(normalizeDepsPathLike(s, testDir))
	}
	inc := konveyor.Incident{URI: uri.URI(s)}
	out, _ := normalizeIncident(inc, testDir)
	return string(out.URI)
}

func normalizeDepsPathLike(s, testDir string) string {
	if s == "" || testDir == "" {
		return NormalizePath(s)
	}
	normalizedTestDir := filepath.ToSlash(testDir)
	s = filepath.ToSlash(s)
	s = strings.ReplaceAll(s, normalizedTestDir, "")
	return NormalizePath(s)
}

func normalizeDepExtrasMap(extras map[string]interface{}, testDir string) map[string]interface{} {
	if extras == nil {
		return nil
	}
	out := make(map[string]interface{}, len(extras))
	for k, v := range extras {
		out[k] = normalizeDepExtraValue(v, testDir)
	}
	return out
}

func normalizeDepExtraValue(v interface{}, testDir string) interface{} {
	switch t := v.(type) {
	case string:
		// Paths in extras (e.g. pomPath) use filesystem paths, not always file:// URIs
		return NormalizePath(normalizeDepsPathLike(t, testDir))
	case map[string]interface{}:
		return normalizeDepExtrasMap(t, testDir)
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, e := range t {
			out[i] = normalizeDepExtraValue(e, testDir)
		}
		return out
	default:
		return v
	}
}

// depLess defines a total ordering for *Dep values so we can sort dependency
// slices in a deterministic way
func depLess(a, b *konveyor.Dep) bool {
	if a == nil {
		return b != nil
	}
	if b == nil {
		return false
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	if a.Version != b.Version {
		return a.Version < b.Version
	}
	if a.Type != b.Type {
		return a.Type < b.Type
	}
	if a.Indirect != b.Indirect {
		return !a.Indirect && b.Indirect
	}
	if a.ResolvedIdentifier != b.ResolvedIdentifier {
		return a.ResolvedIdentifier < b.ResolvedIdentifier
	}
	aLabels := strings.Join(a.Labels, ",")
	bLabels := strings.Join(b.Labels, ",")
	if aLabels != bLabels {
		return aLabels < bLabels
	}
	if a.FileURIPrefix != b.FileURIPrefix {
		return a.FileURIPrefix < b.FileURIPrefix
	}
	return false
}

func sortDepsStable(deps []*konveyor.Dep) {
	sort.SliceStable(deps, func(i, j int) bool {
		return depLess(deps[i], deps[j])
	})
}
