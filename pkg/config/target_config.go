package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// TargetConfig defines how to execute tests (separate from test definitions)
type TargetConfig struct {
	// Type specifies the target: kantra, tackle-hub, tackle-ui, kai-rpc, vscode
	Type string `yaml:"type" validate:"required,oneof=kantra tackle-hub tackle-ui kai-rpc vscode"`

	// Kantra-specific configuration
	Kantra *KantraConfig `yaml:"kantra,omitempty"`

	// Tackle Hub API configuration
	TackleHub *TackleHubConfig `yaml:"tackleHub,omitempty"`

	// Tackle UI configuration
	TackleUI *TackleUIConfig `yaml:"tackleUI,omitempty"`

	// Kai RPC configuration
	KaiRPC *KaiRPCConfig `yaml:"kaiRPC,omitempty"`

	// VSCode extension configuration
	VSCode *VSCodeConfig `yaml:"vscode,omitempty"`
}

// KantraConfig for Kantra CLI execution
type KantraConfig struct {
	BinaryPath    string `yaml:"binaryPath,omitempty"`
	MavenSettings string `yaml:"mavenSettings,omitempty"`
}

// TackleHubConfig for Tackle Hub API execution
type TackleHubConfig struct {
	URL           string `yaml:"url" validate:"required"`
	Username      string `yaml:"username,omitempty"`
	Password      string `yaml:"password,omitempty"`
	Token         string `yaml:"token,omitempty"`
	MavenSettings string `yaml:"mavenSettings,omitempty"`
}

// TackleUIConfig for Tackle UI browser automation
type TackleUIConfig struct {
	URL      string `yaml:"url" validate:"required"`
	Username string `yaml:"username" validate:"required"`
	Password string `yaml:"password" validate:"required"`
	Browser  string `yaml:"browser,omitempty"` // chrome, firefox
	Headless bool   `yaml:"headless,omitempty"`
}

// KaiRPCConfig for Kai analyzer RPC
type KaiRPCConfig struct {
	Host string `yaml:"host" validate:"required"`
	Port int    `yaml:"port" validate:"required"`
}

// VSCodeConfig for VSCode extension execution
type VSCodeConfig struct {
	BinaryPath   string `yaml:"binaryPath,omitempty"` // Path to 'code' binary
	ExtensionID  string `yaml:"extensionId" validate:"required"`
	WorkspaceDir string `yaml:"workspaceDir,omitempty"`
}

// LoadTargetConfig loads target configuration from a file
func LoadTargetConfig(path string) (*TargetConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read target config file %s: %w", path, err)
	}

	var targetConfig TargetConfig
	if err := yaml.Unmarshal(data, &targetConfig); err != nil {
		return nil, fmt.Errorf("failed to parse target config YAML: %w", err)
	}

	return &targetConfig, nil
}
