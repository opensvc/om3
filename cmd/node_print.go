package cmd

import (
	"github.com/spf13/cobra"
)

var nodePrint = &cobra.Command{
	Use:   "print",
	Short: "Print node",
}

func init() {
	nodeCmd.AddCommand(nodePrint)
}
