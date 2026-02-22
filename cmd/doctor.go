package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	warnings := 0

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

	// Check 4: noglob alias recommendation for zsh users
	if runtime.GOOS != "windows" {
		if !checkNoglobAlias() {
			warnings++
		}
	}

	if allPassed {
		if warnings > 0 {
			fmt.Printf("All checks passed! (%d recommendation(s) above)\n", warnings)
		} else {
			fmt.Println("All checks passed!")
		}
	}
	return nil
}

// checkNoglobAlias warns zsh users if they don't have a noglob alias for ai.
// Without it, special characters like ? * # in natural language queries cause
// shell glob expansion errors (e.g. "zsh: no matches found: cpu?").
//
// Returns true if the alias is present or the check is not applicable,
// false if a recommendation was printed.
func checkNoglobAlias() bool {
	shellEnv := os.Getenv("SHELL")
	if shellEnv == "" {
		return true
	}

	base := filepath.Base(shellEnv)
	if base != "zsh" {
		return true
	}

	// Look for a noglob alias in ~/.zshrc rather than spawning an
	// interactive shell (which is slow and may have side effects from
	// plugins like oh-my-zsh).
	home, err := os.UserHomeDir()
	if err != nil {
		printWarning("noglob alias", "Could not determine home directory. For zsh, add to ~/.zshrc: alias ai='noglob ai'")
		return false
	}

	zshrcPath := filepath.Join(home, ".zshrc")
	content, err := os.ReadFile(zshrcPath)
	if err != nil {
		// No .zshrc or unreadable — warn but don't fail.
		printWarning("noglob alias", "Could not read ~/.zshrc. For zsh, add: alias ai='noglob ai'")
		return false
	}

	if hasNoglobAlias(string(content)) {
		printCheck("noglob alias", true, "ai is aliased with noglob (special characters like ? will work)")
		return true
	}

	printWarning("noglob alias",
		"zsh detected but 'ai' is not aliased with noglob in ~/.zshrc. "+
			"Characters like ? * # in queries may cause glob errors.\n"+
			"    Add this to your ~/.zshrc:\n"+
			"      alias ai='noglob ai'\n"+
			"    Then run: source ~/.zshrc")
	return false
}

// noglobAliasPattern matches alias declarations like:
//
//	alias ai='noglob ai'
//	alias ai="noglob ai"
//	alias ai=noglob\ ai
var noglobAliasPattern = regexp.MustCompile(`(?m)^\s*alias\s+ai\s*=\s*['"]?noglob\s`)

// hasNoglobAlias checks whether the given shell config content contains a
// noglob alias for ai. It ignores commented-out lines.
func hasNoglobAlias(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// Skip comments
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if noglobAliasPattern.MatchString(line) {
			return true
		}
	}
	return false
}
