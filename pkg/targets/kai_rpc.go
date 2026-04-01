package targets

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	rpc "github.com/cenkalti/rpc2"
	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/util"
	"gopkg.in/yaml.v2"
)

// KaiRPCTarget implements Target for Kai analyzer RPC
type KaiRPCTarget struct {
	binaryPath         string
	providerConfigPath string
	defaultRulesDir    string
	logFile            string
	verbosity          int
}

// AnalyzeArgs matches the kai-analyzer-rpc service.Args struct
type AnalyzeArgs struct {
	LabelSelector    string   `json:"label_selector,omitempty"`
	IncidentSelector string   `json:"incident_selector,omitempty"`
	IncludedPaths    []string `json:"included_paths,omitempty"`
	ExcludedPaths    []string `json:"excluded_paths,omitempty"`
	ResetCache       bool     `json:"reset_cache,omitempty"`
}

// AnalyzeResponse matches the kai-analyzer-rpc service.Response struct
type AnalyzeResponse struct {
	Rulesets []konveyor.RuleSet `json:"Rulesets"`
}

// NewKaiRPCTarget creates a new Kai RPC target
func NewKaiRPCTarget(cfg *config.KaiRPCConfig) (*KaiRPCTarget, error) {
	if cfg == nil {
		return nil, fmt.Errorf("kai rpc configuration is required")
	}

	// Find binary path
	binaryPath := cfg.BinaryPath
	if binaryPath == "" {
		var err error
		binaryPath, err = exec.LookPath("kai-analyzer")
		if err != nil {
			return nil, fmt.Errorf("kai-analyzer binary not found in PATH: %w", err)
		}
	}

	// Validate provider config exists
	if cfg.ProviderConfigPath == "" {
		return nil, fmt.Errorf("provider config path is required")
	}
	if _, err := os.Stat(cfg.ProviderConfigPath); err != nil {
		return nil, fmt.Errorf("provider config file not found: %w", err)
	}

	verbosity := 0
	if cfg.Verbosity != nil {
		verbosity = *cfg.Verbosity
	}

	return &KaiRPCTarget{
		binaryPath:         binaryPath,
		providerConfigPath: cfg.ProviderConfigPath,
		defaultRulesDir:    cfg.DefaultRulesDir,
		logFile:            cfg.LogFile,
		verbosity:          verbosity,
	}, nil
}

// Name returns the target name
func (k *KaiRPCTarget) Name() string {
	return "kai-rpc"
}

// Execute runs analysis via Kai analyzer RPC
func (k *KaiRPCTarget) Execute(ctx context.Context, test *config.TestDefinition) (*ExecutionResult, error) {
	log := util.GetLogger()
	start := time.Now()

	// Prepare work directory (must be absolute since kai-analyzer runs with cmd.Dir=workDir)
	workDir, err := PrepareWorkDir(test.GetWorkDir(), test.Name)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(workDir) {
		workDir, _ = filepath.Abs(workDir)
	}

	log.Info("Executing Kai RPC analysis", "workDir", workDir)

	// Prepare input (clone git repo if needed)
	inputPath, err := k.prepareInput(ctx, &test.Analysis, workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare input: %w", err)
	}

	// Prepare rules
	preparedRules, err := k.prepareRules(ctx, &test.Analysis, test.GetTestDir(), workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare rules: %w", err)
	}

	// Generate unique pipe path for this execution (platform-specific: Unix socket or Windows named pipe)
	pipePath := generatePipePath()

	// Create output directory
	outputDir := filepath.Join(workDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate provider config with correct source location and analysis mode
	providerConfigPath, err := k.generateProviderConfig(inputPath, workDir, string(test.Analysis.AnalysisMode))
	if err != nil {
		return nil, fmt.Errorf("failed to generate provider config: %w", err)
	}

	// Build rules argument (comma-separated)
	// If no rules specified, use default rulesets if configured
	rulesArg := strings.Join(preparedRules, ",")
	if rulesArg == "" && k.defaultRulesDir != "" {
		if absPath, err := filepath.Abs(k.defaultRulesDir); err == nil {
			rulesArg = absPath
		} else {
			rulesArg = k.defaultRulesDir
		}
		log.Info("Using default rulesets", "path", rulesArg)
	} else if rulesArg == "" {
		return nil, fmt.Errorf("no rules specified and no defaultRulesDir configured; run 'make download-rulesets' and configure defaultRulesDir in target config")
	}

	// Start kai-analyzer server
	log.Info("Starting kai-analyzer server", "pipePath", pipePath)
	cmd, outputLog, err := k.startServer(ctx, pipePath, rulesArg, providerConfigPath, workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to start kai-analyzer server: %w", err)
	}
	defer outputLog.Close()
	defer k.cleanup(cmd, pipePath)

	// Wait for server to be ready and connect
	log.Info("Connecting to kai-analyzer server")
	client, conn, err := k.connectToServer(ctx, pipePath, test.GetTimeout())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to kai-analyzer server: %w", err)
	}
	defer conn.Close()
	defer client.Close()

	// Build analysis args
	args := AnalyzeArgs{
		LabelSelector:    test.Analysis.LabelSelector,
		IncidentSelector: test.Analysis.IncidentSelector,
		ResetCache:       true, // Always reset cache for fresh analysis
	}

	// Call analysis
	log.Info("Calling analysis_engine.Analyze", "args", args)
	var response AnalyzeResponse
	err = client.Call("analysis_engine.Analyze", args, &response)
	if err != nil {
		return nil, fmt.Errorf("RPC call failed: %w", err)
	}

	log.Info("Analysis complete", "rulesetCount", len(response.Rulesets))

	// Write output to file
	outputFile := filepath.Join(outputDir, "output.yaml")
	output, err := yaml.Marshal(response.Rulesets)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal output: %w", err)
	}
	if err := os.WriteFile(outputFile, output, 0644); err != nil {
		return nil, fmt.Errorf("failed to write output file: %w", err)
	}

	duration := time.Since(start)
	result := &ExecutionResult{
		ExitCode:   0,
		Duration:   duration,
		OutputFile: outputFile,
		WorkDir:    workDir,
	}

	return result, nil
}

// startServer starts the kai-analyzer server process
func (k *KaiRPCTarget) startServer(ctx context.Context, pipePath, rules, providerConfig, workDir string) (*exec.Cmd, *os.File, error) {
	args := []string{
		"--server-pipe", pipePath,
		"--rules", rules,
		"--provider-config", providerConfig,
	}
	// Log file path must be absolute since cmd.Dir changes the working directory
	var logFilePath string
	if k.logFile != "" {
		logFilePath = k.logFile
	} else {
		logFilePath = filepath.Join(workDir, "kai-analyzer.log")
	}
	if !filepath.IsAbs(logFilePath) {
		logFilePath, _ = filepath.Abs(logFilePath)
	}
	args = append(args, "--log-file", logFilePath)
	args = append(args, "--verbosity", strconv.Itoa(k.verbosity))

	cmd := exec.CommandContext(ctx, k.binaryPath, args...)
	cmd.Dir = workDir

	// Capture stdout/stderr for debugging
	logFile := filepath.Join(workDir, "kai-analyzer-output.log")
	f, err := os.Create(logFile)
	if err != nil {
		return nil, nil, err
	}
	cmd.Stdout = f
	cmd.Stderr = f

	if err := cmd.Start(); err != nil {
		f.Close()
		return nil, nil, err
	}

	// Return the log file handle — caller closes it after the process exits
	// to avoid closing a file descriptor the child process is writing to.
	return cmd, f, nil
}

// connectToServer waits for the server to be ready and connects
func (k *KaiRPCTarget) connectToServer(ctx context.Context, pipePath string, timeout time.Duration) (*rpc.Client, net.Conn, error) {
	log := util.GetLogger()

	deadline := time.Now().Add(timeout)

	// Wait for pipe/socket to be ready and connect (platform-specific)
	conn, err := waitAndDial(ctx, pipePath, deadline)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to server within %v: %w", timeout, err)
	}

	// Create RPC client with LSP-style codec
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	codec := NewLSPCodec(ctx, reader, writer, log)
	client := rpc.NewClientWithCodec(codec)

	// Wait for "started" notification (with timeout)
	startedCh := make(chan struct{})
	var startedOnce sync.Once
	client.Handle("started", func(client *rpc.Client, args *json.RawMessage, reply *interface{}) error {
		log.Info("Received 'started' notification from server")
		startedOnce.Do(func() { close(startedCh) })
		return nil
	})

	// Start client in background (recover from panics if connection dies)
	go func() {
		defer func() { recover() }()
		client.Run()
	}()

	// Wait for started notification (use the test's configured timeout)
	select {
	case <-startedCh:
		log.Info("kai-analyzer server is ready")
	case <-time.After(timeout):
		client.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("timeout waiting for server 'started' notification after %v", timeout)
	case <-ctx.Done():
		client.Close()
		conn.Close()
		return nil, nil, ctx.Err()
	}

	return client, conn, nil
}

// cleanup stops the server and removes the socket
func (k *KaiRPCTarget) cleanup(cmd *exec.Cmd, pipePath string) {
	log := util.GetLogger()

	if cmd != nil && cmd.Process != nil {
		log.Info("Stopping kai-analyzer server")
		if err := signalGracefulShutdown(cmd.Process); err != nil {
			log.V(1).Info("Failed to signal process for shutdown", "error", err)
		}
		// Wait briefly for graceful shutdown
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	}

	if pipePath != "" {
		removePipe(pipePath)
	}
}

// generateProviderConfig creates a provider config with the correct source location and analysis mode
func (k *KaiRPCTarget) generateProviderConfig(sourcePath, workDir, analysisMode string) (string, error) {
	// Read the template provider config
	data, err := os.ReadFile(k.providerConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to read provider config: %w", err)
	}

	// Parse and update with source location and analysis mode
	var configs []map[string]interface{}
	if err := yaml.Unmarshal(data, &configs); err != nil {
		return "", fmt.Errorf("failed to parse provider config: %w", err)
	}

	// Update initConfig locations and analysis mode
	for i := range configs {
		if initConfigs, ok := configs[i]["initConfig"].([]interface{}); ok {
			for j := range initConfigs {
				if initConfig, ok := initConfigs[j].(map[interface{}]interface{}); ok {
					initConfig["location"] = sourcePath
					if analysisMode != "" {
						initConfig["analysisMode"] = analysisMode
					}
				}
			}
		}
	}

	// Write updated config
	outputPath := filepath.Join(workDir, "provider-config.yaml")
	output, err := yaml.Marshal(configs)
	if err != nil {
		return "", fmt.Errorf("failed to marshal provider config: %w", err)
	}
	if err := os.WriteFile(outputPath, output, 0644); err != nil {
		return "", fmt.Errorf("failed to write provider config: %w", err)
	}

	return outputPath, nil
}

// prepareInput handles git URLs, local paths, and binary files (similar to kantra.go)
func (k *KaiRPCTarget) prepareInput(ctx context.Context, analysis *config.AnalysisConfig, workDir string) (string, error) {
	log := util.GetLogger()

	// Check if it's a binary file
	if IsBinaryFile(analysis.Application) {
		return "", fmt.Errorf("%w: binary analysis not yet supported for kai-rpc", ErrUnsupported)
	}

	// Check if we have parsed Git components
	if analysis.ApplicationGitComponents != nil {
		log.Info("Cloning application repository", "url", analysis.ApplicationGitComponents.URL)
		return CloneGitRepository(ctx, analysis.ApplicationGitComponents, workDir, "source")
	}

	// Local path
	return analysis.Application, nil
}

// prepareRules handles rules that may be Git URLs or local paths
func (k *KaiRPCTarget) prepareRules(ctx context.Context, analysis *config.AnalysisConfig, testDir, workDir string) ([]string, error) {
	return PrepareRules(ctx, analysis.Rules, testDir, workDir)
}

// LSPCodec implements the LSP-style JSON-RPC codec for client-side communication.
// TODO: import codec from github.com/konveyor/kai-analyzer/pkg/codec once the
// upstream module path is fixed (currently declares github.com/konveyor/kai-analyzer
// but lives at github.com/konveyor/kai/kai_analyzer_rpc, making it unresolvable).
type LSPCodec struct {
	reader  *bufio.Reader
	writer  *bufio.Writer
	ctx     context.Context
	log     interface{ Info(string, ...interface{}) }
	seq     uint64
	seqLock sync.Mutex

	// Current response being read
	currentResponse *lspResponse
}

type lspRequest struct {
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      *uint64     `json:"id,omitempty"`
	JSONRPC string      `json:"jsonrpc"`
}

type lspResponse struct {
	ID      *uint64          `json:"id,omitempty"`
	Result  *json.RawMessage `json:"result,omitempty"`
	Error   interface{}      `json:"error,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  *json.RawMessage `json:"params,omitempty"`
	JSONRPC string           `json:"jsonrpc"`
}

// NewLSPCodec creates a new LSP-style codec for client communication
func NewLSPCodec(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer, log interface{ Info(string, ...interface{}) }) *LSPCodec {
	return &LSPCodec{
		reader: reader,
		writer: writer,
		ctx:    ctx,
		log:    log,
	}
}

// ReadHeader reads the JSON-RPC header
func (c *LSPCodec) ReadHeader(req *rpc.Request, resp *rpc.Response) error {
	// Read Content-Length header
	header, err := c.reader.ReadString('\n')
	if err != nil {
		return err
	}

	// Parse Content-Length
	var contentLength int
	if _, err := fmt.Sscanf(header, "Content-Length: %d", &contentLength); err != nil {
		return fmt.Errorf("invalid header: %s", header)
	}

	// Skip empty line
	if _, err := c.reader.ReadString('\n'); err != nil {
		return err
	}

	// Read body - must use io.ReadFull to ensure all bytes are read
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.reader, body); err != nil {
		return err
	}

	// Parse JSON
	var response lspResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return err
	}

	c.currentResponse = &response

	// Check if it's a request/notification (has Method) or response (has ID without Method)
	if response.Method != "" {
		// It's a notification or request from server
		req.Method = response.Method
		req.Seq = 0 // Notifications have no ID
		if response.ID != nil {
			req.Seq = *response.ID
		}
	} else if response.ID != nil {
		// It's a response
		resp.Seq = *response.ID
		if response.Error != nil {
			if errStr, ok := response.Error.(string); ok {
				resp.Error = errStr
			} else {
				resp.Error = "RPC error"
			}
		}
	}

	return nil
}

// ReadRequestBody reads the request body (for handling server->client requests)
func (c *LSPCodec) ReadRequestBody(v interface{}) error {
	if v == nil || c.currentResponse == nil || c.currentResponse.Params == nil {
		return nil
	}
	return json.Unmarshal(*c.currentResponse.Params, v)
}

// ReadResponseBody reads the response body
func (c *LSPCodec) ReadResponseBody(v interface{}) error {
	if v == nil || c.currentResponse == nil || c.currentResponse.Result == nil {
		return nil
	}
	return json.Unmarshal(*c.currentResponse.Result, v)
}

// WriteRequest writes a JSON-RPC request
func (c *LSPCodec) WriteRequest(req *rpc.Request, v interface{}) error {
	c.seqLock.Lock()
	c.seq++
	seq := c.seq
	c.seqLock.Unlock()

	request := lspRequest{
		Method:  req.Method,
		Params:  []interface{}{v},
		ID:      &seq,
		JSONRPC: "2.0",
	}

	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := c.writer.WriteString(header); err != nil {
		return err
	}
	if _, err := c.writer.Write(body); err != nil {
		return err
	}
	return c.writer.Flush()
}

// WriteResponse writes a JSON-RPC response (for responding to server requests)
func (c *LSPCodec) WriteResponse(resp *rpc.Response, v interface{}) error {
	// Client typically doesn't need to respond to server
	return nil
}

// Close closes the codec
func (c *LSPCodec) Close() error {
	return nil
}
