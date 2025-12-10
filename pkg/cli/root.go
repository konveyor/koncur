package cli

import (
	"fmt"
	"os"

	"github.com/konveyor/test-harness/pkg/util"
	"github.com/spf13/cobra"
)

var (
	verbose bool
)

// NewRootCmd creates the root command
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "koncur",
		Short: "Test harness for Konveyor tools",
		Long: `Koncur - A test harness for running and validating end-to-end tests
for Konveyor tools (Kantra, Tackle, Kai).

Koncur concurs with your expected results!`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			util.InitLogger(verbose)
		},
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	// Add subcommands
	rootCmd.AddCommand(NewRunCmd())
	rootCmd.AddCommand(NewValidateCmd())
	rootCmd.AddCommand(NewGenerateCmd())
	rootCmd.AddCommand(NewCleanCmd())
	rootCmd.AddCommand(NewConfigCmd())

	return rootCmd
}

// Execute runs the root command
func Execute() {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
