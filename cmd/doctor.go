package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

func printWarning(label string, detail string) {
	fmt.Printf("  %s %s: %s\n", color.YellowString("⚠"), label, detail)
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

	allPassed := true

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
		allPassed = false
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

	// Check 4: noglob alias for zsh users
	if runtime.GOOS != "windows" {
		checkNoglobAlias(&allPassed)
	}

	if allPassed {
		fmt.Println("All checks passed!")
	}
	return nil
}

// checkNoglobAlias warns zsh users if they don't have a noglob alias for ai.
// Without it, special characters like ? * # in natural language queries cause
// shell glob expansion errors (e.g. "zsh: no matches found: cpu?").
func checkNoglobAlias(allPassed *bool) {
	shellEnv := os.Getenv("SHELL")
	if shellEnv == "" {
		return
	}

	base := filepath.Base(shellEnv)
	if base != "zsh" {
		return
	}

	// Check if the user's interactive zsh has a noglob alias for ai.
	// We launch zsh as an interactive login shell so it sources .zshrc/.zprofile.
	out, err := exec.Command(shellEnv, "-i", "-c", "alias ai 2>/dev/null").Output()
	if err != nil {
		// If the command fails, we can't determine the alias state; show a hint.
		*allPassed = false
		printWarning("noglob alias", "Could not detect shell aliases. For zsh, add to ~/.zshrc: alias ai='noglob ai'")
		return
	}

	aliasOutput := strings.TrimSpace(string(out))

	// zsh alias output looks like: ai='noglob ai' or ai=noglob ai
	if strings.Contains(aliasOutput, "noglob") {
		printCheck("noglob alias", true, "ai is aliased with noglob (special characters like ? will work)")
		return
	}

	*allPassed = false
	printWarning("noglob alias",
		"zsh detected but 'ai' is not aliased with noglob. "+
			"Characters like ? * # in queries will cause glob errors.\n"+
			"    Add this to your ~/.zshrc:\n"+
			"      alias ai='noglob ai'\n"+
			"    Then run: source ~/.zshrc")
}
