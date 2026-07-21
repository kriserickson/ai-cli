package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/shell"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show current configuration status",
		RunE:  runStatus,
	})
}

func runStatus(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	configPath, err := config.Path()
	if err != nil {
		return err
	}

	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	logPath := filepath.Join(configDir, "llm.log")

	fmt.Printf("ai %s\n", Version)

	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Config:   %s  %s\n", configPath, color.GreenString("✓ exists"))
	} else {
		fmt.Printf("Config:   %s  %s\n", configPath, color.RedString("✗ not found"))
	}

	if _, err := os.Stat(logPath); err == nil {
		fmt.Printf("Log:      %s  %s\n", logPath, color.GreenString("✓ exists"))
	} else {
		fmt.Printf("Log:      %s  — not created yet\n", logPath)
	}

	fmt.Printf("Provider: %s\n", cfg.Provider.Default)
	fmt.Printf("Model:    %s\n", cfg.Provider.Model)
	if cfg.Provider.ModelLight != "" {
		provider := cfg.Provider.ProviderLight
		if provider == "" {
			provider = cfg.Provider.Default
		}
		fmt.Printf("Light:    %s / %s\n", provider, cfg.Provider.ModelLight)
	}
	if cfg.Provider.ModelHigh != "" {
		provider := cfg.Provider.ProviderHigh
		if provider == "" {
			provider = cfg.Provider.Default
		}
		fmt.Printf("High:     %s / %s\n", provider, cfg.Provider.ModelHigh)
	}
	fmt.Printf("Upgrade:  %t\n", cfg.Provider.UpgradeModelOnFail)
	fmt.Printf("Debug:    %s\n", cfg.Debug)
	if cfg.Debug == config.DebugFile {
		if cfg.DebugLogPayloads {
			fmt.Printf("Log Data: full payloads enabled\n")
		} else {
			fmt.Printf("Log Data: metadata only\n")
		}
	}

	shellInfo := shell.Detect()
	fmt.Printf("Shell:    %s  (%s)\n", shellInfo.Shell, shellInfo.Version)

	if cfg.Provider.Default == config.ProviderLocal {
		fmt.Printf("Base URL: %s\n", cfg.Provider.Local.BaseURL)
		if cfg.Provider.Local.APIKey != "" {
			fmt.Printf("API Key:  %s\n", color.GreenString(maskKey(cfg.Provider.Local.APIKey)))
		} else {
			fmt.Printf("API Key:  %s\n", color.YellowString("not set (optional for local)"))
		}
	} else {
		var apiKey string
		switch cfg.Provider.Default {
		case config.ProviderOpenAI:
			apiKey = cfg.Provider.OpenAI.APIKey
		case config.ProviderOpenRouter:
			apiKey = cfg.Provider.OpenRouter.APIKey
		}

		if apiKey == "" {
			fmt.Printf("API Key:  %s\n", color.RedString("not set"))
		} else {
			fmt.Printf("API Key:  %s\n", color.GreenString(maskKey(apiKey)))
		}
	}

	return nil
}
