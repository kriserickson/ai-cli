package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "0.1.0"

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("ai-cli v%s\n", Version)
		},
	})
}
