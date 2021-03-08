package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints"
)

var cfgLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "Print the selected objects path",
	Run:   cfgLsCmdRun,
}

func init() {
	cfgCmd.AddCommand(cfgLsCmd)
}

func cfgLsCmdRun(cmd *cobra.Command, args []string) {
	entrypoints.List{
		ObjectSelector: mergeSelector(cfgSelectorFlag, "cfg", "**"),
		Format:         formatFlag,
		Color:          colorFlag,
		Server:         serverFlag,
	}.Do()
}
