package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
	"opensvc.com/opensvc/core/object"
)

var (
	nodeFreezeNodeFlag  string
	nodeFreezeLocalFlag bool
	nodeFreezeWatchFlag bool
)

var nodeFreezeCmd = &cobra.Command{
	Use:   "freeze",
	Short: "Freeze the selected objects.",
	Run:   nodeFreezeCmdRun,
}

func init() {
	nodeCmd.AddCommand(nodeFreezeCmd)
	nodeFreezeCmd.Flags().BoolVarP(&nodeFreezeLocalFlag, "local", "", false, "Freeze inline the selected local instances.")
	nodeFreezeCmd.Flags().BoolVarP(&nodeFreezeWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func nodeFreezeCmdRun(cmd *cobra.Command, args []string) {
	a := action.NodeAction{
		Action: action.Action{
			NodeSelector: nodeFreezeNodeFlag,
			Action:       "freeze",
			Target:       "frozen",
			Watch:        nodeFreezeWatchFlag,
			Format:       formatFlag,
			Color:        colorFlag,
		},
		Node: object.NodeAction{
			Run: func() (interface{}, error) {
				return nil, object.NewNode().Freeze()
			},
		},
	}
	action.Do(a)
}
