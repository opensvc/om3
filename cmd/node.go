package cmd

import (
	"github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage a opensvc cluster node",
}

func init() {
	rootCmd.AddCommand(nodeCmd)
}
