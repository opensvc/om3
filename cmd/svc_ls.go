package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints"
)

var svcLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "Print the selected objects path",
	Run:   svcLsCmdRun,
}

func init() {
	svcCmd.AddCommand(svcLsCmd)
}

func svcLsCmdRun(cmd *cobra.Command, args []string) {
	entrypoints.List{
		ObjectSelector: mergeSelector(svcSelectorFlag, "svc", "**"),
		Format:         formatFlag,
		Color:          colorFlag,
	}.Do()
}
