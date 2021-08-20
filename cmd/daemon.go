package cmd

import (
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the opensvc daemon",
}

func init() {
	root.AddCommand(daemonCmd)
}
