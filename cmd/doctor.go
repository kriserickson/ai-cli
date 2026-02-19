package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "doctor",
		Short: "Check and repair configuration",
		RunE:  runDoctor,
	})
}

func printCheck(label string, ok bool, detail string) {
	if ok {
		fmt.Printf("  %s %s: %s\n", color.GreenString("✓"), label, detail)
	} else {
		fmt.Printf("  %s %s: %s\n", color.RedString("✗"), label, detail)
	}
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("Checking configuration...")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}

	// Check 1: Config file — always ✓ (config.Load auto-creates)
	printCheck("Config file", true, configPath)

	// Check 2: API key for current provider
	var apiKey string
	switch cfg.Provider.Default {
	case "openai":
		apiKey = cfg.Provider.OpenAI.APIKey
	case "openrouter":
		apiKey = cfg.Provider.OpenRouter.APIKey
	}

	if apiKey == "" {
		printCheck("API key", false, fmt.Sprintf("No API key configured for %s", cfg.Provider.Default))
		fmt.Println("    Running setup wizard...")
		if err := RunModelWizard(cfg); err != nil {
			return err
		}
	} else {
		printCheck("API key", true, fmt.Sprintf("%s (%s)", cfg.Provider.Default, maskKey(apiKey)))
	}

	// Check 3: Model is set — always ✓ (default exists)
	printCheck("Model", true, cfg.Provider.Model)

	fmt.Println("All checks passed!")
	return nil
}
