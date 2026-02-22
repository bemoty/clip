package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information for clip",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("clip %s (%s, built %s)\n", version, commit, date)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
