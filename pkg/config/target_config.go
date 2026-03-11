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

	// Container image overrides for kantra's container mode (--run-local=false)
	// These correspond to kantra's environment variables for provider images.
	RunnerImage          string `yaml:"runnerImage,omitempty"`
	JavaProviderImage    string `yaml:"javaProviderImage,omitempty"`
	GenericProviderImage string `yaml:"genericProviderImage,omitempty"`
	CsharpProviderImage  string `yaml:"csharpProviderImage,omitempty"`
}

// TackleHubConfig for Tackle Hub API execution
type TackleHubConfig struct {
	URL           string `yaml:"url" validate:"required"`
	Username      string `yaml:"username,omitempty"`
	Password      string `yaml:"password,omitempty"`
	Token         string `yaml:"token,omitempty"`
	MavenSettings string `yaml:"mavenSettings,omitempty"`
	Insecure      bool   `yaml:"insecure,omitempty"`

	// Container image overrides for Tackle Hub components.
	// When set, koncur patches the Tackle CR on the cluster before running tests.
	Images *TackleHubImages `yaml:"images,omitempty"`

	// Kubernetes configuration for patching the Tackle CR.
	// Only needed when images are specified.
	Namespace string `yaml:"namespace,omitempty"` // Default: konveyor-tackle
	CRName    string `yaml:"crName,omitempty"`    // Default: tackle
}

// TackleHubImages defines container image overrides for Tackle Hub components.
// These map to fields in the Tackle Custom Resource spec.
type TackleHubImages struct {
	Hub             string `yaml:"hub,omitempty"`             // hub_image_fqin
	Analyzer        string `yaml:"analyzer,omitempty"`        // analyzer_fqin
	JavaProvider    string `yaml:"javaProvider,omitempty"`    // provider_java_image_fqin
	GenericProvider string `yaml:"genericProvider,omitempty"` // provider_python_image_fqin + provider_nodejs_image_fqin
	CsharpProvider  string `yaml:"csharpProvider,omitempty"`  // provider_c_sharp_image_fqin
	Runner          string `yaml:"runner,omitempty"`          // kantra_fqin
	DiscoveryAddon  string `yaml:"discoveryAddon,omitempty"`  // language_discovery_fqin
	PlatformAddon   string `yaml:"platformAddon,omitempty"`   // platform_fqin
}

// GetNamespace returns the namespace, defaulting to konveyor-tackle
func (c *TackleHubConfig) GetNamespace() string {
	if c.Namespace != "" {
		return c.Namespace
	}
	return "konveyor-tackle"
}

// GetCRName returns the Tackle CR name, defaulting to tackle
func (c *TackleHubConfig) GetCRName() string {
	if c.CRName != "" {
		return c.CRName
	}
	return "tackle"
}

// HasImageOverrides returns true if any image overrides are configured
func (c *TackleHubConfig) HasImageOverrides() bool {
	if c.Images == nil {
		return false
	}
	return c.Images.Hub != "" ||
		c.Images.Analyzer != "" ||
		c.Images.JavaProvider != "" ||
		c.Images.GenericProvider != "" ||
		c.Images.CsharpProvider != "" ||
		c.Images.Runner != "" ||
		c.Images.DiscoveryAddon != "" ||
		c.Images.PlatformAddon != ""
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
