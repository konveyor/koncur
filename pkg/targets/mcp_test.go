package targets

import (
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
