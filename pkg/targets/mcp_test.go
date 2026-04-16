package targets

import (
	"context"
	"testing"

	"github.com/konveyor/test-harness/pkg/config"
)

func TestNewMCPTarget(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.MCPConfig
		wantErr    bool
		errContain string
	}{
		{
			name:       "nil config",
			cfg:        nil,
			wantErr:    true,
			errContain: "mcp configuration is required",
		},
		{
			name:       "no binary path or endpoint",
			cfg:        &config.MCPConfig{},
			wantErr:    true,
			errContain: "either binaryPath or endpoint must be set",
		},
		{
			name: "both binary path and endpoint",
			cfg: &config.MCPConfig{
				BinaryPath: "/usr/local/bin/analyzer-mcp",
				Endpoint:   "http://localhost:8080",
			},
			wantErr:    true,
			errContain: "binaryPath and endpoint are mutually exclusive",
		},
		{
			name: "valid stdio config",
			cfg: &config.MCPConfig{
				BinaryPath:     "/usr/local/bin/analyzer-mcp",
				ProviderConfig: "/path/to/config.yaml",
				Rules:          "/path/to/rules",
			},
			wantErr: false,
		},
		{
			name: "valid http config",
			cfg: &config.MCPConfig{
				Endpoint: "http://localhost:8080",
			},
			wantErr: false,
		},
		{
			name: "http config with token",
			cfg: &config.MCPConfig{
				Endpoint: "http://localhost:8080",
				Token:    "my-token",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := NewMCPTarget(tt.cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewMCPTarget() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContain != "" {
				if !contains(err.Error(), tt.errContain) {
					t.Errorf("Expected error to contain %q, got %q", tt.errContain, err.Error())
				}
			}

			if !tt.wantErr && target == nil {
				t.Error("Expected non-nil target")
			}
		})
	}
}

func TestMCPTarget_Name(t *testing.T) {
	target := &MCPTarget{}
	if target.Name() != "mcp" {
		t.Errorf("Expected name 'mcp', got %q", target.Name())
	}
}

func TestMCPTarget_Execute_Stdio_NonexistentBinary(t *testing.T) {
	target := &MCPTarget{
		binaryPath:     "/nonexistent/analyzer-mcp",
		providerConfig: "/path/to/config.yaml",
		rules:          "/path/to/rules",
	}

	test := &config.TestDefinition{
		Name: "test-mcp-stdio",
		Analysis: config.AnalysisConfig{
			LabelSelector: "konveyor.io/target=eap8",
			Application:   "/path/to/app",
		},
	}

	_, err := target.Execute(context.Background(), test)
	if err == nil {
		t.Error("Expected error for nonexistent binary")
	}
}

func TestMCPTarget_Execute_HTTP_NoServer(t *testing.T) {
	target := &MCPTarget{
		endpoint: "http://127.0.0.1:19999",
	}

	test := &config.TestDefinition{
		Name: "test-mcp-http",
		Analysis: config.AnalysisConfig{
			LabelSelector: "konveyor.io/target=eap8",
			Application:   "/path/to/app",
		},
	}

	_, err := target.Execute(context.Background(), test)
	if err == nil {
		t.Error("Expected error when no server is running")
	}
}

func TestMCPTarget_buildStdioArgs(t *testing.T) {
	tests := []struct {
		name          string
		target        *MCPTarget
		test          *config.TestDefinition
		expectContain []string
		expectLen     int
	}{
		{
			name: "all args",
			target: &MCPTarget{
				rules:          "/path/to/rules",
				providerConfig: "/path/to/config.yaml",
			},
			test: &config.TestDefinition{
				Analysis: config.AnalysisConfig{
					LabelSelector: "konveyor.io/target=eap8",
				},
			},
			expectContain: []string{
				"--rules", "/path/to/rules",
				"--provider-config", "/path/to/config.yaml",
				"--label-selector", "konveyor.io/target=eap8",
			},
			expectLen: 6,
		},
		{
			name: "no label selector",
			target: &MCPTarget{
				rules:          "/path/to/rules",
				providerConfig: "/path/to/config.yaml",
			},
			test: &config.TestDefinition{
				Analysis: config.AnalysisConfig{},
			},
			expectContain: []string{
				"--rules", "/path/to/rules",
				"--provider-config", "/path/to/config.yaml",
			},
			expectLen: 4,
		},
		{
			name:   "no config values",
			target: &MCPTarget{},
			test: &config.TestDefinition{
				Analysis: config.AnalysisConfig{},
			},
			expectLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.target.buildStdioArgs(tt.test)

			if len(args) != tt.expectLen {
				t.Errorf("Expected %d args, got %d: %v", tt.expectLen, len(args), args)
			}

			for _, expected := range tt.expectContain {
				found := false
				for _, arg := range args {
					if arg == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected arg %q not found in %v", expected, args)
				}
			}
		})
	}
}

func TestBuildAnalyzeToolArgs(t *testing.T) {
	tests := []struct {
		name   string
		test   *config.TestDefinition
		expect map[string]interface{}
	}{
		{
			name: "with label and incident selector",
			test: &config.TestDefinition{
				Analysis: config.AnalysisConfig{
					LabelSelector:    "konveyor.io/target=eap8",
					IncidentSelector: "package=io.konveyor",
				},
			},
			expect: map[string]interface{}{
				"label_selector":    "konveyor.io/target=eap8",
				"incident_selector": "package=io.konveyor",
				"reset_cache":       true,
			},
		},
		{
			name: "label selector only",
			test: &config.TestDefinition{
				Analysis: config.AnalysisConfig{
					LabelSelector: "konveyor.io/target=eap8",
				},
			},
			expect: map[string]interface{}{
				"label_selector": "konveyor.io/target=eap8",
				"reset_cache":    true,
			},
		},
		{
			name: "no selectors",
			test: &config.TestDefinition{
				Analysis: config.AnalysisConfig{},
			},
			expect: map[string]interface{}{
				"reset_cache": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := buildAnalyzeToolArgs(tt.test)

			if len(args) != len(tt.expect) {
				t.Errorf("Expected %d args, got %d: %v", len(tt.expect), len(args), args)
			}

			for k, v := range tt.expect {
				got, ok := args[k]
				if !ok {
					t.Errorf("Expected key %q not found in args", k)
					continue
				}
				if got != v {
					t.Errorf("Expected args[%q] = %v, got %v", k, v, got)
				}
			}
		})
	}
}
