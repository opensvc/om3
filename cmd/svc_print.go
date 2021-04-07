package cmd

import (
	"github.com/spf13/cobra"
)

var svcPrintCmd = &cobra.Command{
	Use:     "print",
	Short:   "print information about the object",
	Aliases: []string{"prin", "pri", "pr"},
}

func init() {
	svcCmd.AddCommand(svcPrintCmd)
}
