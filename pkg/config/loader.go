package config

import (
	"fmt"
	"os"
	"path/filepath"

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

	// If the expected output specifies a file, load it (unless skipped)
	if test.Expect.Output.File != "" && !skipExpectedOutput {
		// Resolve the expected output file path relative to the test file's directory
		expectedOutputPath := test.Expect.Output.File
		if !filepath.IsAbs(expectedOutputPath) {
			testDir := filepath.Dir(path)
			expectedOutputPath = filepath.Join(testDir, expectedOutputPath)
		}

		rulesets, err := LoadExpectedOutput(expectedOutputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load expected output from %s: %w", test.Expect.Output.File, err)
		}

		test.Expect.Output.Result = rulesets
		test.Expect.Output.File = "" // Clear the file path since we've loaded it
	}

	return &test, nil
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
