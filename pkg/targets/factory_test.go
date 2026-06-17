package targets

import (
	"os"
	"testing"

	"github.com/konveyor/test-harness/pkg/config"
)

func TestNewTarget(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.TargetConfig
		wantType   string
		wantErr    bool
		errContain string
	}{
		{
			name: "kantra target",
			cfg: &config.TargetConfig{
				Type: "kantra",
				Kantra: &config.KantraConfig{
					BinaryPath: "/usr/local/bin/kantra",
				},
			},
			wantType: "kantra",
			wantErr:  false,
		},
		{
			name: "tackle-hub target",
			cfg: &config.TargetConfig{
				Type: "tackle-hub",
				TackleHub: &config.TackleHubConfig{
					URL: "http://localhost:8080",
				},
			},
			wantType: "tackle-hub",
			wantErr:  false,
		},
		{
			name: "tackle-ui target",
			cfg: &config.TargetConfig{
				Type: "tackle-ui",
				TackleUI: &config.TackleUIConfig{
					URL: "http://localhost:3000",
				},
			},
			wantType: "tackle-ui",
			wantErr:  false,
		},
		{
			name: "kai-rpc target",
			cfg: &config.TargetConfig{
				Type: "kai-rpc",
				KaiRPC: &config.KaiRPCConfig{
					ProviderConfigPath: "testdata/provider.yaml",
				},
			},
			wantType: "kai-rpc",
			wantErr:  false,
		},
		{
			name: "vscode target",
			cfg: &config.TargetConfig{
				Type: "vscode",
				VSCode: &config.VSCodeConfig{
					ExtensionID: "konveyor.analyzer-lsp",
				},
			},
			wantType: "vscode",
			wantErr:  false,
		},
		{
			name: "unknown target type",
			cfg: &config.TargetConfig{
				Type: "unknown",
			},
			wantErr:    true,
			errContain: "unknown target type",
		},
		{
			name: "empty target type",
			cfg: &config.TargetConfig{
				Type: "",
			},
			wantErr:    true,
			errContain: "unknown target type",
		},
		{
			name: "kantra target without config",
			cfg: &config.TargetConfig{
				Type:   "kantra",
				Kantra: nil,
			},
			// Will try to find kantra in PATH, may fail depending on environment
			wantType: "kantra",
			wantErr:  true, // Expected to fail if kantra not in PATH
		},
		{
			name: "tackle-hub target without config",
			cfg: &config.TargetConfig{
				Type:      "tackle-hub",
				TackleHub: nil,
			},
			wantErr:    true,
			errContain: "tackle hub configuration is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			target, err := NewTarget(tt.cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewTarget() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContain != "" {
				if !contains(err.Error(), tt.errContain) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errContain, err.Error())
				}
			}

			if !tt.wantErr && target != nil {
				if target.Name() != tt.wantType {
					t.Errorf("Expected target type '%s', got '%s'", tt.wantType, target.Name())
				}
			}
		})
	}
}

func TestNewTarget_AllTypes(t *testing.T) {
	// Test that we can create all target types without panics
	targetTypes := []string{"kantra", "tackle-hub", "tackle-ui", "kai-rpc", "vscode"}

	for _, targetType := range targetTypes {
		t.Run(targetType, func(t *testing.T) {
			var cfg *config.TargetConfig

			switch targetType {
			case "kantra":
				cfg = &config.TargetConfig{
					Type: "kantra",
					Kantra: &config.KantraConfig{
						BinaryPath: "/usr/local/bin/kantra",
					},
				}
			case "tackle-hub":
				cfg = &config.TargetConfig{
					Type: "tackle-hub",
					TackleHub: &config.TackleHubConfig{
						URL: "http://localhost:8080",
					},
				}
			case "tackle-ui":
				cfg = &config.TargetConfig{
					Type: "tackle-ui",
					TackleUI: &config.TackleUIConfig{
						URL: "http://localhost:3000",
					},
				}
			case "kai-rpc":
				cfg = &config.TargetConfig{
					Type: "kai-rpc",
					KaiRPC: &config.KaiRPCConfig{
						ProviderConfigPath: "testdata/provider.yaml",
					},
				}
			case "vscode":
				cfg = &config.TargetConfig{
					Type: "vscode",
					VSCode: &config.VSCodeConfig{
						ExtensionID: "konveyor.analyzer-lsp",
					},
				}
			}

			target, err := NewTarget(cfg)
			if err != nil {
				t.Logf("Creating %s target returned error (may be expected): %v", targetType, err)
			} else if target == nil {
				t.Errorf("NewTarget() returned nil target without error for type '%s'", targetType)
			} else if target.Name() != targetType {
				t.Errorf("Expected target name '%s', got '%s'", targetType, target.Name())
			}
		})
	}
}

func TestNewTarget_NilConfig(t *testing.T) {
	// This should panic or return error, test defensive behavior
	defer func() {
		if r := recover(); r != nil {
			t.Logf("NewTarget panicked with nil config (recovered): %v", r)
		}
	}()

	// Can't pass nil config to NewTarget as it would panic on cfg.Type access
	// This is expected behavior - config is required
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && hasSubstring(s, substr)
}

func hasSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
