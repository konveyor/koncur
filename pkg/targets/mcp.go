package targets

import (
	"context"
	"fmt"

	"github.com/konveyor/test-harness/pkg/config"
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
	return nil, fmt.Errorf("mcp target not yet implemented")
}
