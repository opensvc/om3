package cmd

import (
	"github.com/spf13/cobra"
)

var nodeScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan node",
}

func init() {
	nodeCmd.AddCommand(nodeScanCmd)
}
