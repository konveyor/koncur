package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"gopkg.in/yaml.v3"
)

// Load reads and parses a test definition from a YAML file
func Load(path string) (*TestDefinition, error) {
	return LoadWithOptions(path, false)
}

// LoadWithOptions reads and parses a test definition with options
// skipExpectedOutput: if true, don't try to load the expected output file (useful for generation)
func LoadWithOptions(path string, skipExpectedOutput bool) (*TestDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read test file %s: %w", path, err)
	}
	var test TestDefinition
	if err := yaml.Unmarshal(data, &test); err != nil {
		return nil, fmt.Errorf("failed to parse test YAML: %w", err)
	}
	skipAll, onlyTargets, _ := parseSkipCommentPreamble(data)
	test.commentSkipAll = skipAll
	test.commentSkipTargets = onlyTargets

	// Store the absolute path to the test file
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	test.SetTestFilePath(absPath)

	// Parse Git URLs in the analysis configuration
	test.Analysis.ParseGitURLs()

	// If the expected output specifies a file, load it (unless skipped)
	if test.Expect.Output.File != "" && !skipExpectedOutput {
		// Resolve the expected output file path relative to the test file's directory
		expectedOutputPath := test.Expect.Output.File
		if !filepath.IsAbs(expectedOutputPath) {
			testDir := filepath.Dir(path)
			expectedOutputPath = filepath.Join(testDir, expectedOutputPath)
		}

		// Store the resolved absolute path
		absExpectedPath, err := filepath.Abs(expectedOutputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for expected output: %w", err)
		}
		test.Expect.Output.ResolvedFilePath = absExpectedPath

		rulesets, err := LoadExpectedOutput(expectedOutputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load expected output from %s: %w", test.Expect.Output.File, err)
		}

		test.Expect.Output.Result = rulesets
	}

	return &test, nil
}

// --- # SKIPPED preamble (first skipCommentScanBytes of test.yaml) -----------------
//
// Parsed in LoadWithOptions alongside YAML. Used by koncur run / generate via
// (*TestDefinition).ShouldSkipForTarget.
//
//   - # SKIPPED: … or # SKIPPED … with no leading target token → skip for all targets.
//   - # SKIPPED: kantra or # SKIPPED: tackle-hub (comma-separated) → skip only for those targets.
//   - "hub" is accepted as an alias for tackle-hub.

const skipCommentScanBytes = 500

func parseSkipCommentPreamble(data []byte) (skipAll bool, onlyTargets []string, found bool) {
	scan := data
	if len(scan) > skipCommentScanBytes {
		scan = scan[:skipCommentScanBytes]
	}

	var merged map[string]struct{}
	lineFound := false
	sawUnconditional := false

	for _, rawLine := range strings.Split(string(scan), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if isYAMLDocumentPreambleLine(line) {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			break
		}
		body := strings.TrimSpace(line[1:])
		lowerBody := strings.ToLower(body)
		const skippedWord = "skipped"
		if !strings.HasPrefix(lowerBody, skippedWord) {
			continue
		}
		lineFound = true
		rest := body[len(skippedWord):]
		rest = strings.TrimSpace(strings.TrimLeft(rest, ": \t"))
		if rest == "" {
			sawUnconditional = true
			continue
		}
		lineTargets, lineAll := parseSkipRestTargets(rest)
		if lineAll {
			sawUnconditional = true
		} else {
			if merged == nil {
				merged = make(map[string]struct{})
			}
			for _, t := range lineTargets {
				merged[t] = struct{}{}
			}
		}
	}

	if !lineFound {
		return false, nil, false
	}
	if sawUnconditional {
		return true, nil, true
	}
	if len(merged) == 0 {
		return true, nil, true
	}
	out := make([]string, 0, len(merged))
	for t := range merged {
		out = append(out, t)
	}
	sort.Strings(out)
	return false, out, true
}

// isYAMLDocumentPreambleLine reports lines that may appear before the logical start of
// YAML content (still part of the header / stream markers).
func isYAMLDocumentPreambleLine(line string) bool {
	if line == "---" {
		return true
	}
	return strings.HasPrefix(line, "%YAML")
}

func parseSkipRestTargets(rest string) (targets []string, skipAll bool) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil, true
	}
	segs := strings.Split(rest, ",")
	var out []string
	for _, seg := range segs {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		fields := strings.Fields(seg)
		if len(fields) == 0 {
			continue
		}
		w := strings.ToLower(fields[0])
		switch w {
		case "kantra":
			out = append(out, "kantra")
		case "tackle-hub":
			out = append(out, "tackle-hub")
		case "hub":
			out = append(out, "tackle-hub")
		default:
			return nil, true
		}
	}
	if len(out) == 0 {
		return nil, true
	}
	seen := make(map[string]struct{})
	dedup := make([]string, 0, len(out))
	for _, t := range out {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		dedup = append(dedup, t)
	}
	return dedup, false
}

// ShouldSkipForTarget reports whether this test should be skipped for the given target
// type (e.g. "kantra", "tackle-hub"). It honors yaml skipped: and # SKIPPED comment lines
// parsed in LoadWithOptions.
func (t *TestDefinition) ShouldSkipForTarget(targetType string) bool {
	if t.Skipped {
		return true
	}
	if t.commentSkipAll {
		return true
	}
	for _, x := range t.commentSkipTargets {
		if x == targetType {
			return true
		}
	}
	return false
}

// LoadExpectedOutput reads and parses expected RuleSets from a YAML file
func LoadExpectedOutput(path string) ([]konveyor.RuleSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read expected output file: %w", err)
	}

	var rulesets []konveyor.RuleSet
	if err := yaml.Unmarshal(data, &rulesets); err != nil {
		return nil, fmt.Errorf("failed to parse expected output YAML: %w", err)
	}

	return rulesets, nil
}
