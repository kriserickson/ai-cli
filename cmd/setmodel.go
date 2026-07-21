package cmd

import (
	"github.com/spf13/cobra"

	"github.com/kriserickson/ai-cli/internal/config"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:       "set-model [light|default|high]",
		Short:     "Interactively configure a provider and model tier",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{config.ModelLevelLight, config.ModelLevelDefault, config.ModelLevelHigh},
		RunE:      runSetModel,
	})
}

func runSetModel(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	level := config.ModelLevelDefault
	if len(args) > 0 {
		level = args[0]
	}
	return RunModelWizardForLevel(cfg, level)
}
