package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints"
)

var nodeEventsCmd = &cobra.Command{
	Use:     "events",
	Short:   "Print the node event stream",
	Aliases: []string{"eve", "even", "event"},
	Run:     nodeEventsCmdRun,
}

func init() {
	nodeCmd.AddCommand(nodeEventsCmd)
}

func nodeEventsCmdRun(cmd *cobra.Command, args []string) {
	e := entrypoints.Events{
		Format: formatFlag,
		Color:  colorFlag,
		Server: serverFlag,
	}
	e.Do()
}
