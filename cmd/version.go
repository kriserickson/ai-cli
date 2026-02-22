package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "dev"

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("ai %s\n", Version)
		},
	})
}
