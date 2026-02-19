package cmd

import (
	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "set-model",
		Short: "Interactively select a provider and model",
		RunE:  runSetModel,
	})
}

func runSetModel(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return RunModelWizard(cfg)
}
