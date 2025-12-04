package config

import (
	"time"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/konveyor/analyzer-lsp/provider"
)

// TestDefinition represents a single test case
type TestDefinition struct {
	Name        string `yaml:"name" validate:"required"`
	Description string `yaml:"description,omitempty"`

	// Analysis configuration - what to analyze
	Analysis AnalysisConfig `yaml:"analysis" validate:"required"`

	// Optional execution settings
	Timeout *Duration `yaml:"timeout,omitempty"`
	WorkDir string    `yaml:"workDir,omitempty"`

	// Validation configuration
	Expect ExpectConfig `yaml:"expect" validate:"required"`
}

// AnalysisConfig defines what to analyze
type AnalysisConfig struct {
	// Application is either a file path or git repository URL
	Application   string                `yaml:"application" validate:"required"`
	LabelSelector string                `yaml:"labelSelector,omitempty"`
	AnalysisMode  provider.AnalysisMode `yaml:"analysisMode" validate:"required"`
}

// ExpectConfig defines expected outcomes
type ExpectConfig struct {
	ExitCode int            `yaml:"exitCode"`
	Output   ExpectedOutput `yaml:"output" validate:"required"`
}

// ExpectedOutput is a union type for expected output
// Either Result or File must be set, but not both
type ExpectedOutput struct {
	// Result contains inline expected RuleSets
	Result []konveyor.RuleSet `yaml:"result,omitempty"`

	// File path to YAML file containing expected RuleSets
	File string `yaml:"file,omitempty"`
}

// Duration is a wrapper around time.Duration that supports YAML unmarshaling
type Duration struct {
	time.Duration
}

// UnmarshalYAML implements custom unmarshaling for duration strings
func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

// MarshalYAML implements custom marshaling for Duration
func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// GetTimeout returns the timeout duration with a default
func (td *TestDefinition) GetTimeout() time.Duration {
	if td.Timeout != nil {
		return td.Timeout.Duration
	}
	return 5 * time.Minute // Default timeout
}

// GetWorkDir returns the work directory with a default
func (td *TestDefinition) GetWorkDir() string {
	if td.WorkDir != "" {
		return td.WorkDir
	}
	// Use .koncur/output in current directory instead of /tmp for podman compatibility
	return ".koncur/output"
}
