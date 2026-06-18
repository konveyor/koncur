package targets

import (
	"context"
	"testing"

	"github.com/konveyor/test-harness/pkg/config"
)

func TestNewKaiRPCTarget(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.KaiRPCConfig
		wantErr    bool
		errContain string
	}{
		{
			name:       "nil config",
			cfg:        nil,
			wantErr:    true,
			errContain: "kai rpc configuration is required",
		},
		{
			name: "valid config",
			cfg: &config.KaiRPCConfig{
				Host: "localhost",
				Port: 9090,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := NewKaiRPCTarget(tt.cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewKaiRPCTarget() error = %v, wantErr %v", err, tt.wantErr)
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

func TestKaiRPCTarget_Name(t *testing.T) {
	target := &KaiRPCTarget{}
	if target.Name() != "kai-rpc" {
		t.Errorf("Expected name 'kai-rpc', got %q", target.Name())
	}
}

func TestKaiRPCTarget_Execute_NoServer(t *testing.T) {
	target := &KaiRPCTarget{
		host: "127.0.0.1",
		port: 19998,
	}

	test := &config.TestDefinition{
		Name: "test-kai-rpc",
		Analysis: config.AnalysisConfig{
			LabelSelector: "konveyor.io/target=eap8",
			Application:   "/path/to/app",
		},
	}

	_, err := target.Execute(context.Background(), test)
	if err == nil {
		t.Error("Expected error when no server is running")
	}
	if !contains(err.Error(), "failed to connect") {
		t.Errorf("Expected connection error, got: %v", err)
	}
}

func TestBuildRPCAnalyzeArgs(t *testing.T) {
	tests := []struct {
		name   string
		test   *config.TestDefinition
		expect rpcAnalyzeArgs
	}{
		{
			name: "with label and incident selector",
			test: &config.TestDefinition{
				Analysis: config.AnalysisConfig{
					LabelSelector:    "konveyor.io/target=eap8",
					IncidentSelector: "package=io.konveyor",
				},
			},
			expect: rpcAnalyzeArgs{
				LabelSelector:    "konveyor.io/target=eap8",
				IncidentSelector: "package=io.konveyor",
				ResetCache:       true,
			},
		},
		{
			name: "label selector only",
			test: &config.TestDefinition{
				Analysis: config.AnalysisConfig{
					LabelSelector: "konveyor.io/target=eap8",
				},
			},
			expect: rpcAnalyzeArgs{
				LabelSelector: "konveyor.io/target=eap8",
				ResetCache:    true,
			},
		},
		{
			name: "no selectors",
			test: &config.TestDefinition{
				Analysis: config.AnalysisConfig{},
			},
			expect: rpcAnalyzeArgs{
				ResetCache: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRPCAnalyzeArgs(tt.test)

			if got.LabelSelector != tt.expect.LabelSelector {
				t.Errorf("LabelSelector = %q, want %q", got.LabelSelector, tt.expect.LabelSelector)
			}
			if got.IncidentSelector != tt.expect.IncidentSelector {
				t.Errorf("IncidentSelector = %q, want %q", got.IncidentSelector, tt.expect.IncidentSelector)
			}
			if got.ResetCache != tt.expect.ResetCache {
				t.Errorf("ResetCache = %v, want %v", got.ResetCache, tt.expect.ResetCache)
			}
		})
	}
}
