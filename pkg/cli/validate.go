package cli

import (
	"fmt"

	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/util"
	"github.com/spf13/cobra"
)

// NewValidateCmd creates the validate command
func NewValidateCmd() *cobra.Command {
	validateCmd := &cobra.Command{
		Use:   "validate <test-file>",
		Short: "Validate a test definition",
		Long:  `Check if a test definition is valid without running it.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			testFile := args[0]
			log := util.GetLogger()

			log.Info("Validating test definition", "file", testFile)

			// Load test definition
			test, err := config.Load(testFile)
			if err != nil {
				return err
			}

			// Validate test definition
			if err := config.Validate(test); err != nil {
				return err
			}

			fmt.Printf("✓ Test definition is valid: %s\n", test.Name)
			return nil
		},
	}

	return validateCmd
}
