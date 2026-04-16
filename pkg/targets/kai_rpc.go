package targets

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	rpc "github.com/cenkalti/rpc2"
	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/util"
	"gopkg.in/yaml.v3"
)

// rpcAnalyzeArgs mirrors service.Args from kai-analyzer.
// Defined here to avoid importing the full kai-analyzer module.
type rpcAnalyzeArgs struct {
	LabelSelector    string   `json:"label_selector,omitempty"`
	IncidentSelector string   `json:"incident_selector,omitempty"`
	IncludedPaths    []string `json:"included_paths,omitempty"`
	ExcludedPaths    []string `json:"excluded_paths,omitempty"`
	ResetCache       bool     `json:"reset_cache,omitempty"`
}

// rpcAnalyzeResponse mirrors service.Response from kai-analyzer.
type rpcAnalyzeResponse struct {
	Rulesets []konveyor.RuleSet
}

// KaiRPCTarget implements Target for Kai analyzer RPC
type KaiRPCTarget struct {
	host string
	port int
}

// NewKaiRPCTarget creates a new Kai RPC target
func NewKaiRPCTarget(cfg *config.KaiRPCConfig) (*KaiRPCTarget, error) {
	if cfg == nil {
		return nil, fmt.Errorf("kai rpc configuration is required")
	}

	return &KaiRPCTarget{
		host: cfg.Host,
		port: cfg.Port,
	}, nil
}

// Name returns the target name
func (k *KaiRPCTarget) Name() string {
	return "kai-rpc"
}

// Execute runs analysis via Kai analyzer RPC
func (k *KaiRPCTarget) Execute(ctx context.Context, test *config.TestDefinition) (*ExecutionResult, error) {
	log := util.GetLogger()
	log.Info("Executing KaiRPC analysis", "test", test.Name)

	start := time.Now()

	workDir, err := PrepareWorkDir(test.GetWorkDir(), test.Name)
	if err != nil {
		return nil, err
	}

	// Connect to RPC server
	addr := fmt.Sprintf("%s:%d", k.host, k.port)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to KaiRPC server at %s: %w", addr, err)
	}
	defer conn.Close()

	client := rpc.NewClient(conn)
	defer client.Close()

	// Build analysis args
	args := buildRPCAnalyzeArgs(test)

	// Call Analyze
	var response rpcAnalyzeResponse
	if err := client.CallWithContext(ctx, "analysis_engine.Analyze", args, &response); err != nil {
		return nil, fmt.Errorf("RPC Analyze call failed: %w", err)
	}

	// Write rulesets as YAML to output.yaml
	outputFile := filepath.Join(workDir, "output.yaml")
	yamlData, err := yaml.Marshal(response.Rulesets)
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

// buildRPCAnalyzeArgs constructs analysis args from a test definition
func buildRPCAnalyzeArgs(test *config.TestDefinition) rpcAnalyzeArgs {
	return rpcAnalyzeArgs{
		LabelSelector:    test.Analysis.LabelSelector,
		IncidentSelector: test.Analysis.IncidentSelector,
		ResetCache:       true,
	}
}
