package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kriserickson/ai-cli/internal/memory"
)

func init() {
	memoryCmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage stored memories for contextual command generation",
	}

	memoryCmd.AddCommand(
		&cobra.Command{
			Use:   "add <keyword> <content...>",
			Short: "Add a memory",
			Args:  cobra.MinimumNArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				keyword := args[0]
				content := strings.Join(args[1:], " ")
				if err := memory.Add(keyword, content); err != nil {
					return err
				}
				fmt.Printf("Memory %q added.\n", keyword)
				return nil
			},
		},
		&cobra.Command{
			Use:   "remove <keyword>",
			Short: "Remove a memory",
			Args:  cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				if err := memory.Remove(args[0]); err != nil {
					return err
				}
				fmt.Printf("Memory %q removed.\n", args[0])
				return nil
			},
		},
		&cobra.Command{
			Use:   subCmdList,
			Short: "List all memories",
			Args:  cobra.NoArgs,
			RunE: func(_ *cobra.Command, _ []string) error {
				return listMemories()
			},
		},
	)

	rootCmd.AddCommand(memoryCmd)
}

func listMemories() error {
	entries, err := memory.List()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No memories stored.")
		return nil
	}
	for _, e := range entries {
		fmt.Printf("  %-20s %s\n", e.Keyword, e.Content)
	}
	return nil
}
