package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
)

var (
	svcFreezeNodeFlag  string
	svcFreezeLocalFlag bool
	svcFreezeWatchFlag bool
)

var svcFreezeCmd = &cobra.Command{
	Use:   "freeze",
	Short: "Freeze the selected objects",
	Run:   svcFreezeCmdRun,
}

func init() {
	svcCmd.AddCommand(svcFreezeCmd)
	svcFreezeCmd.Flags().BoolVarP(&svcFreezeLocalFlag, "local", "", false, "Freeze inline the selected local instances.")
	svcFreezeCmd.Flags().BoolVarP(&svcFreezeWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func svcFreezeCmdRun(cmd *cobra.Command, args []string) {
	a := action.ObjectAction{
		ObjectSelector: mergeSelector(svcSelectorFlag, "svc", ""),
		NodeSelector:   svcFreezeNodeFlag,
		Action:         "freeze",
		Method:         "Freeze",
		Target:         "frozen",
		Watch:          svcFreezeWatchFlag,
		Format:         formatFlag,
		Color:          colorFlag,
	}
	action.Do(a)
}
