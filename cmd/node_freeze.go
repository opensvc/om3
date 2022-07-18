package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

var (
	nodeFreezeNodeFlag  string
	nodeFreezeLocalFlag bool
	nodeFreezeWatchFlag bool
)

var nodeFreezeCmd = &cobra.Command{
	Use:   "freeze",
	Short: "freeze the selected objects.",
	Run:   nodeFreezeCmdRun,
}

func init() {
	nodeCmd.AddCommand(nodeFreezeCmd)
	nodeFreezeCmd.Flags().StringVarP(&nodeFreezeNodeFlag, "node", "", "", "the nodes to execute the action on")
	nodeFreezeCmd.Flags().BoolVarP(&nodeFreezeLocalFlag, "local", "", false, "Freeze inline the selected local instances.")
	nodeFreezeCmd.Flags().BoolVarP(&nodeFreezeWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func nodeFreezeCmdRun(_ *cobra.Command, _ []string) {
	nodeaction.New(
		nodeaction.WithRemoteNodes(nodeFreezeNodeFlag),
		nodeaction.WithRemoteAction("freeze"),
		nodeaction.WithAsyncTarget("frozen"),
		nodeaction.WithAsyncWatch(nodeFreezeWatchFlag),
		nodeaction.WithFormat(formatFlag),
		nodeaction.WithColor(colorFlag),
		nodeaction.WithLocal(nodeFreezeLocalFlag),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return nil, n.Freeze()
		}),
	).Do()
}
