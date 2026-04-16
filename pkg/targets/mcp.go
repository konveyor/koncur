package targets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/util"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"
)

// MCPTarget implements Target for MCP analyzer server
type MCPTarget struct {
	binaryPath     string
	providerConfig string
	rules          string
	endpoint       string
	token          string
}

// NewMCPTarget creates a new MCP target
func NewMCPTarget(cfg *config.MCPConfig) (*MCPTarget, error) {
	if cfg == nil {
		return nil, fmt.Errorf("mcp configuration is required")
	}

	hasBinary := cfg.BinaryPath != ""
	hasEndpoint := cfg.Endpoint != ""

	if !hasBinary && !hasEndpoint {
		return nil, fmt.Errorf("either binaryPath or endpoint must be set")
	}
	if hasBinary && hasEndpoint {
		return nil, fmt.Errorf("binaryPath and endpoint are mutually exclusive")
	}

	return &MCPTarget{
		binaryPath:     cfg.BinaryPath,
		providerConfig: cfg.ProviderConfig,
		rules:          cfg.Rules,
		endpoint:       cfg.Endpoint,
		token:          cfg.Token,
	}, nil
}

// Name returns the target name
func (m *MCPTarget) Name() string {
	return "mcp"
}

// Execute runs analysis via MCP analyzer server
func (m *MCPTarget) Execute(ctx context.Context, test *config.TestDefinition) (*ExecutionResult, error) {
	log := util.GetLogger()
	log.Info("Executing MCP analysis", "test", test.Name)

	start := time.Now()

	workDir, err := PrepareWorkDir(test.GetWorkDir(), test.Name)
	if err != nil {
		return nil, err
	}

	// Connect to MCP server
	var transport mcpsdk.Transport
	if m.binaryPath != "" {
		args := m.buildStdioArgs(test)
		cmd := exec.CommandContext(ctx, m.binaryPath, args...)
		transport = &mcpsdk.CommandTransport{Command: cmd}
	} else {
		transport = &mcpsdk.StreamableClientTransport{
			Endpoint: m.endpoint,
		}
	}

	client := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "koncur",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}
	defer session.Close()

	// Call the analyze tool
	toolArgs := buildAnalyzeToolArgs(test)
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "analyze",
		Arguments: toolArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to call analyze tool: %w", err)
	}

	if result.IsError {
		msg := "analyze tool returned error"
		if len(result.Content) > 0 {
			if tc, ok := result.Content[0].(*mcpsdk.TextContent); ok {
				msg = tc.Text
			}
		}
		return nil, fmt.Errorf("%s", msg)
	}

	// Parse JSON response into []konveyor.RuleSet
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("analyze tool returned no content")
	}

	tc, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		return nil, fmt.Errorf("unexpected content type from analyze tool")
	}

	var rulesets []konveyor.RuleSet
	if err := json.Unmarshal([]byte(tc.Text), &rulesets); err != nil {
		return nil, fmt.Errorf("failed to parse analysis results: %w", err)
	}

	// Write rulesets as YAML to output.yaml
	outputFile := filepath.Join(workDir, "output.yaml")
	yamlData, err := yaml.Marshal(rulesets)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rulesets to YAML: %w", err)
	}

	if err := os.WriteFile(outputFile, yamlData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write output file: %w", err)
	}

	duration := time.Since(start)
	execResult := &ExecutionResult{
		Duration:   duration,
		OutputFile: outputFile,
		WorkDir:    workDir,
	}

	LogResult(log, execResult)
	return execResult, nil
}

// buildStdioArgs constructs CLI arguments for the analyzer-mcp subprocess
func (m *MCPTarget) buildStdioArgs(test *config.TestDefinition) []string {
	var args []string

	if m.rules != "" {
		args = append(args, "--rules", m.rules)
	}
	if m.providerConfig != "" {
		args = append(args, "--provider-config", m.providerConfig)
	}
	if test.Analysis.LabelSelector != "" {
		args = append(args, "--label-selector", test.Analysis.LabelSelector)
	}

	return args
}

// buildAnalyzeToolArgs constructs the MCP tool arguments for the analyze call
func buildAnalyzeToolArgs(test *config.TestDefinition) map[string]interface{} {
	args := map[string]interface{}{
		"reset_cache": true,
	}

	if test.Analysis.LabelSelector != "" {
		args["label_selector"] = test.Analysis.LabelSelector
	}
	if test.Analysis.IncidentSelector != "" {
		args["incident_selector"] = test.Analysis.IncidentSelector
	}

	return args
}
