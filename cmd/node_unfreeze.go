package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
)

var (
	nodeUnfreezeNodeFlag  string
	nodeUnfreezeLocalFlag bool
	nodeUnfreezeWatchFlag bool
)

var nodeUnfreezeCmd = &cobra.Command{
	Use:     "unfreeze",
	Aliases: []string{"thaw"},
	Short:   "Unfreeze the selected objects.",
	Run:     nodeUnfreezeCmdRun,
}

func init() {
	nodeCmd.AddCommand(nodeUnfreezeCmd)
	nodeUnfreezeCmd.Flags().BoolVarP(&nodeFreezeLocalFlag, "local", "", false, "Freeze inline the selected local instances.")
	nodeUnfreezeCmd.Flags().BoolVarP(&nodeFreezeWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func nodeUnfreezeCmdRun(cmd *cobra.Command, args []string) {
	action.NodeAction{
		NodeSelector: nodeUnfreezeNodeFlag,
		Action:       "unfreeze",
		Method:       "Unfreeze",
		Target:       "thawed",
		Watch:        nodeUnfreezeWatchFlag,
		Format:       formatFlag,
		Color:        colorFlag,
	}.Do()
}
