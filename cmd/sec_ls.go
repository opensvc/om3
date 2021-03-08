package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints"
)

var secLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "Print the selected objects path",
	Run:   secLsCmdRun,
}

func init() {
	secCmd.AddCommand(secLsCmd)
}

func secLsCmdRun(cmd *cobra.Command, args []string) {
	entrypoints.List{
		ObjectSelector: mergeSelector(secSelectorFlag, "sec", "**"),
		Format:         formatFlag,
		Color:          colorFlag,
	}.Do()
}
