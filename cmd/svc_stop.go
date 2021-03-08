package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
)

var (
	svcStopNodeFlag  string
	svcStopLocalFlag bool
	svcStopWatchFlag bool
)

var svcStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the selected objects",
	Run:   svcStopCmdRun,
}

func init() {
	svcCmd.AddCommand(svcStopCmd)
	svcStopCmd.Flags().BoolVarP(&svcStopLocalFlag, "local", "", false, "Stop inline the selected local instances.")
	svcStopCmd.Flags().BoolVarP(&svcStopWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func svcStopCmdRun(cmd *cobra.Command, args []string) {
	a := action.ObjectAction{
		ObjectSelector: mergeSelector(svcSelectorFlag, "svc", ""),
		NodeSelector:   svcStopNodeFlag,
		Local:          svcStopLocalFlag,
		Action:         "stop",
		Method:         "Stop",
		Target:         "stopped",
		Watch:          svcStopWatchFlag,
		Format:         formatFlag,
		Color:          colorFlag,
	}
	action.Do(a)
}
