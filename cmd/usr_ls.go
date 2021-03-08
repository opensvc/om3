package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints"
)

var usrLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "Print the selected objects path",
	Run:   usrLsCmdRun,
}

func init() {
	usrCmd.AddCommand(usrLsCmd)
}

func usrLsCmdRun(cmd *cobra.Command, args []string) {
	entrypoints.List{
		ObjectSelector: mergeSelector(usrSelectorFlag, "usr", "**"),
		Format:         formatFlag,
		Color:          colorFlag,
	}.Do()
}
