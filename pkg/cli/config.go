package cli

import (
	"fmt"
	"os"
	"path/filepath"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/konveyor/analyzer-lsp/provider"
	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/util"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	configOutputFile string
	configType       string
	configNonInteractive bool
)

// NewConfigCmd creates the config command with subcommands
func NewConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage koncur configurations",
		Long:  `Create and manage target and test configurations for koncur.`,
	}

	// Add subcommands
	configCmd.AddCommand(NewConfigTargetCmd())
	configCmd.AddCommand(NewConfigTestCmd())

	return configCmd
}

// NewConfigTargetCmd creates the config target command
func NewConfigTargetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "target",
		Short: "Generate a target configuration file",
		Long: `Interactively create a target configuration file for koncur.

Supported target types:
  - kantra: Kantra CLI execution
  - tackle-hub: Tackle Hub API execution
  - tackle-ui: Tackle UI browser automation (not implemented)
  - kai-rpc: Kai analyzer RPC (not implemented)
  - vscode: VSCode extension execution (not implemented)`,
		RunE: runConfigTarget,
	}

	cmd.Flags().StringVarP(&configOutputFile, "output", "o", "", "Output file path (default: .koncur/config/target-<type>.yaml)")
	cmd.Flags().StringVarP(&configType, "type", "t", "", "Target type (kantra, tackle-hub, tackle-ui, kai-rpc, vscode)")

	return cmd
}

// NewConfigTestCmd creates the config test command
func NewConfigTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Generate a test configuration file",
		Long: `Interactively create a test definition file for koncur.

A test definition specifies:
  - The application to analyze
  - Analysis parameters (label selector, mode)
  - Expected results (exit code, output)`,
		RunE: runConfigTest,
	}

	cmd.Flags().StringVarP(&configOutputFile, "output", "o", "", "Output file path (default: ./test.yaml)")

	return cmd
}

func runConfigTarget(cmd *cobra.Command, args []string) error {
	log := util.GetLogger()

	// Prompt for target type if not provided
	targetType := configType
	if targetType == "" {
		prompt := promptui.Select{
			Label: "Select target type",
			Items: []string{"kantra", "tackle-hub", "tackle-ui", "kai-rpc", "vscode"},
		}
		_, result, err := prompt.Run()
		if err != nil {
			return fmt.Errorf("failed to select target type: %w", err)
		}
		targetType = result
	}

	// Create target config based on type
	var targetConfig *config.TargetConfig
	var err error

	switch targetType {
	case "kantra":
		targetConfig, err = createKantraConfig()
	case "tackle-hub":
		targetConfig, err = createTackleHubConfig()
	case "tackle-ui":
		targetConfig, err = createTackleUIConfig()
	case "kai-rpc":
		targetConfig, err = createKaiRPCConfig()
	case "vscode":
		targetConfig, err = createVSCodeConfig()
	default:
		return fmt.Errorf("unsupported target type: %s", targetType)
	}

	if err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}

	// Determine output file
	outputFile := configOutputFile
	if outputFile == "" {
		outputFile = fmt.Sprintf(".koncur/config/target-%s.yaml", targetType)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(outputFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(targetConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.Info("Target configuration created", "file", outputFile, "type", targetType)
	fmt.Printf("✓ Created target configuration: %s\n", outputFile)

	return nil
}

func runConfigTest(cmd *cobra.Command, args []string) error {
	log := util.GetLogger()

	// Prompt for test details
	testConfig, err := createTestConfig()
	if err != nil {
		return fmt.Errorf("failed to create test config: %w", err)
	}

	// Determine output file
	outputFile := configOutputFile
	if outputFile == "" {
		outputFile = "./test.yaml"
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(outputFile)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Marshal to YAML
	data, err := yaml.Marshal(testConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.Info("Test configuration created", "file", outputFile)
	fmt.Printf("✓ Created test configuration: %s\n", outputFile)

	return nil
}

// createKantraConfig creates a Kantra target configuration interactively
func createKantraConfig() (*config.TargetConfig, error) {
	kantraConfig := &config.KantraConfig{}

	// Prompt for binary path (optional)
	prompt := promptui.Prompt{
		Label:   "Kantra binary path (optional, press Enter to use PATH)",
		Default: "",
	}
	binaryPath, err := prompt.Run()
	if err != nil && err != promptui.ErrInterrupt {
		return nil, err
	}
	if binaryPath != "" {
		kantraConfig.BinaryPath = binaryPath
	}

	// Prompt for Maven settings (optional)
	prompt = promptui.Prompt{
		Label:   "Maven settings.xml path (optional, press Enter to skip)",
		Default: "",
	}
	mavenSettings, err := prompt.Run()
	if err != nil && err != promptui.ErrInterrupt {
		return nil, err
	}
	if mavenSettings != "" {
		kantraConfig.MavenSettings = mavenSettings
	}

	return &config.TargetConfig{
		Type:   "kantra",
		Kantra: kantraConfig,
	}, nil
}

// createTackleHubConfig creates a Tackle Hub target configuration interactively
func createTackleHubConfig() (*config.TargetConfig, error) {
	tackleHubConfig := &config.TackleHubConfig{}

	// Prompt for URL (required)
	prompt := promptui.Prompt{
		Label:   "Tackle Hub URL",
		Default: "http://localhost:8081",
	}
	url, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	tackleHubConfig.URL = url

	// Prompt for authentication method
	authPrompt := promptui.Select{
		Label: "Authentication method",
		Items: []string{"Token", "Username/Password", "None"},
	}
	_, authMethod, err := authPrompt.Run()
	if err != nil {
		return nil, err
	}

	switch authMethod {
	case "Token":
		prompt = promptui.Prompt{
			Label: "API Token",
			Mask:  '*',
		}
		token, err := prompt.Run()
		if err != nil {
			return nil, err
		}
		tackleHubConfig.Token = token

	case "Username/Password":
		prompt = promptui.Prompt{
			Label:   "Username",
			Default: "admin",
		}
		username, err := prompt.Run()
		if err != nil {
			return nil, err
		}
		tackleHubConfig.Username = username

		prompt = promptui.Prompt{
			Label: "Password",
			Mask:  '*',
		}
		password, err := prompt.Run()
		if err != nil {
			return nil, err
		}
		tackleHubConfig.Password = password
	}

	// Prompt for Maven settings (optional)
	prompt = promptui.Prompt{
		Label:   "Maven settings.xml path (optional, press Enter to skip)",
		Default: "",
	}
	mavenSettings, err := prompt.Run()
	if err != nil && err != promptui.ErrInterrupt {
		return nil, err
	}
	if mavenSettings != "" {
		tackleHubConfig.MavenSettings = mavenSettings
	}

	return &config.TargetConfig{
		Type:      "tackle-hub",
		TackleHub: tackleHubConfig,
	}, nil
}

// createTackleUIConfig creates a Tackle UI target configuration interactively
func createTackleUIConfig() (*config.TargetConfig, error) {
	fmt.Println("⚠ Warning: Tackle UI target is not yet implemented")

	tackleUIConfig := &config.TackleUIConfig{}

	prompt := promptui.Prompt{
		Label:   "Tackle UI URL",
		Default: "http://localhost:8080",
	}
	url, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	tackleUIConfig.URL = url

	prompt = promptui.Prompt{
		Label:   "Username",
		Default: "admin",
	}
	username, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	tackleUIConfig.Username = username

	prompt = promptui.Prompt{
		Label: "Password",
		Mask:  '*',
	}
	password, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	tackleUIConfig.Password = password

	browserPrompt := promptui.Select{
		Label: "Browser",
		Items: []string{"chrome", "firefox"},
	}
	_, browser, err := browserPrompt.Run()
	if err != nil {
		return nil, err
	}
	tackleUIConfig.Browser = browser

	headlessPrompt := promptui.Select{
		Label: "Headless mode",
		Items: []string{"true", "false"},
	}
	_, headless, err := headlessPrompt.Run()
	if err != nil {
		return nil, err
	}
	tackleUIConfig.Headless = headless == "true"

	return &config.TargetConfig{
		Type:     "tackle-ui",
		TackleUI: tackleUIConfig,
	}, nil
}

// createKaiRPCConfig creates a Kai RPC target configuration interactively
func createKaiRPCConfig() (*config.TargetConfig, error) {
	fmt.Println("⚠ Warning: Kai RPC target is not yet implemented")

	kaiRPCConfig := &config.KaiRPCConfig{}

	prompt := promptui.Prompt{
		Label:   "Kai RPC Host",
		Default: "localhost",
	}
	host, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	kaiRPCConfig.Host = host

	prompt = promptui.Prompt{
		Label:   "Kai RPC Port",
		Default: "8080",
	}
	portStr, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	kaiRPCConfig.Port = port

	return &config.TargetConfig{
		Type:   "kai-rpc",
		KaiRPC: kaiRPCConfig,
	}, nil
}

// createVSCodeConfig creates a VSCode target configuration interactively
func createVSCodeConfig() (*config.TargetConfig, error) {
	fmt.Println("⚠ Warning: VSCode target is not yet implemented")

	vscodeConfig := &config.VSCodeConfig{}

	prompt := promptui.Prompt{
		Label:   "VSCode binary path (optional, press Enter to use PATH)",
		Default: "",
	}
	binaryPath, err := prompt.Run()
	if err != nil && err != promptui.ErrInterrupt {
		return nil, err
	}
	if binaryPath != "" {
		vscodeConfig.BinaryPath = binaryPath
	}

	prompt = promptui.Prompt{
		Label:   "Extension ID",
		Default: "konveyor.konveyor-analyzer",
	}
	extensionID, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	vscodeConfig.ExtensionID = extensionID

	prompt = promptui.Prompt{
		Label:   "Workspace directory (optional, press Enter to skip)",
		Default: "",
	}
	workspaceDir, err := prompt.Run()
	if err != nil && err != promptui.ErrInterrupt {
		return nil, err
	}
	if workspaceDir != "" {
		vscodeConfig.WorkspaceDir = workspaceDir
	}

	return &config.TargetConfig{
		Type:   "vscode",
		VSCode: vscodeConfig,
	}, nil
}

// createTestConfig creates a test configuration interactively
func createTestConfig() (*config.TestDefinition, error) {
	testConfig := &config.TestDefinition{}

	// Prompt for test name
	prompt := promptui.Prompt{
		Label: "Test name",
	}
	name, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	testConfig.Name = name

	// Prompt for description (optional)
	prompt = promptui.Prompt{
		Label:   "Test description (optional, press Enter to skip)",
		Default: "",
	}
	description, err := prompt.Run()
	if err != nil && err != promptui.ErrInterrupt {
		return nil, err
	}
	testConfig.Description = description

	// Analysis section
	testConfig.Analysis = config.AnalysisConfig{}

	// Prompt for application path
	prompt = promptui.Prompt{
		Label: "Application path or git URL",
	}
	application, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	testConfig.Analysis.Application = application

	// Prompt for label selector (optional)
	prompt = promptui.Prompt{
		Label:   "Label selector (optional, press Enter to skip)",
		Default: "",
	}
	labelSelector, err := prompt.Run()
	if err != nil && err != promptui.ErrInterrupt {
		return nil, err
	}
	testConfig.Analysis.LabelSelector = labelSelector

	// Prompt for analysis mode
	modePrompt := promptui.Select{
		Label: "Analysis mode",
		Items: []string{"source-only", "full"},
	}
	_, mode, err := modePrompt.Run()
	if err != nil {
		return nil, err
	}
	testConfig.Analysis.AnalysisMode = provider.AnalysisMode(mode)

	// Expect section
	testConfig.Expect = config.ExpectConfig{
		ExitCode: 0,
		Output: config.ExpectedOutput{
			Result: []konveyor.RuleSet{},
		},
	}

	fmt.Println("\n✓ Test configuration created")
	fmt.Println("  Note: You'll need to run 'koncur generate' to populate expected outputs")

	return testConfig, nil
}
