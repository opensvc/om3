package cmd

import (
	"github.com/spf13/cobra"
)

var svcEditCmd = &cobra.Command{
	Use:     "edit",
	Short:   "edit information about the object",
	Aliases: []string{"edi", "ed"},
}

func init() {
	svcCmd.AddCommand(svcEditCmd)
}
