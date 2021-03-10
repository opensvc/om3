package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints"
)

var (
	volLsLocalFlag bool
)

var volLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "Print the selected objects path",
	Run:   volLsCmdRun,
}

func init() {
	volCmd.AddCommand(volLsCmd)
	volLsCmd.Flags().BoolVarP(&volLsLocalFlag, "local", "", false, "select only local instances")
}

func volLsCmdRun(cmd *cobra.Command, args []string) {
	entrypoints.List{
		ObjectSelector: mergeSelector(volSelectorFlag, "vol", "**"),
		Format:         formatFlag,
		Color:          colorFlag,
		Local:          volLsLocalFlag,
	}.Do()
}
