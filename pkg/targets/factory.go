package targets

import (
	"fmt"

	"github.com/konveyor/test-harness/pkg/config"
)

// NewTarget creates a target instance based on the configuration
func NewTarget(cfg *config.TargetConfig) (Target, error) {
	switch cfg.Type {
	case "kantra":
		return NewKantraTarget(cfg.Kantra)
	case "tackle-hub":
		return NewTackleHubTarget(cfg.TackleHub)
	case "tackle-ui":
		return NewTackleUITarget(cfg.TackleUI)
	case "kai-rpc":
		return NewKaiRPCTarget(cfg.KaiRPC)
	case "vscode":
		return NewVSCodeTarget(cfg.VSCode)
	default:
		return nil, fmt.Errorf("unknown target type: %s", cfg.Type)
	}
}
