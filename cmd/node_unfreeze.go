package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
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
	nodeUnfreezeCmd.Flags().StringVarP(&nodeUnfreezeNodeFlag, "node", "", "", "the nodes to execute the action on")
	nodeUnfreezeCmd.Flags().BoolVarP(&nodeUnfreezeLocalFlag, "local", "", false, "freeze inline the selected local instances")
	nodeUnfreezeCmd.Flags().BoolVarP(&nodeUnfreezeWatchFlag, "watch", "w", false, "watch the monitor changes")
}

func nodeUnfreezeCmdRun(_ *cobra.Command, _ []string) {
	nodeaction.New(
		nodeaction.WithRemoteNodes(nodeUnfreezeNodeFlag),
		nodeaction.WithRemoteAction("unfreeze"),
		nodeaction.WithAsyncTarget("thawed"),
		nodeaction.WithAsyncWatch(nodeUnfreezeWatchFlag),
		nodeaction.WithFormat(formatFlag),
		nodeaction.WithColor(colorFlag),
		nodeaction.WithLocal(nodeUnfreezeLocalFlag),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return nil, object.NewNode().Unfreeze()
		}),
	).Do()
}
