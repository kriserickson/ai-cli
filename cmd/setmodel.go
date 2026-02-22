package cmd

import (
	"github.com/spf13/cobra"

	"github.com/kriserickson/ai-cli/internal/config"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "set-model",
		Short: "Interactively select a provider and model",
		RunE:  runSetModel,
	})
}

func runSetModel(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return RunModelWizard(cfg)
}
