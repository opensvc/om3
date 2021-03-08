package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
)

var (
	svcUnfreezeNodeFlag  string
	svcUnfreezeLocalFlag bool
	svcUnfreezeWatchFlag bool
)

var svcUnfreezeCmd = &cobra.Command{
	Use:     "unfreeze",
	Aliases: []string{"thaw"},
	Short:   "Unfreeze the selected objects",
	Run:     svcUnfreezeCmdRun,
}

func init() {
	svcCmd.AddCommand(svcUnfreezeCmd)
	svcUnfreezeCmd.Flags().BoolVarP(&svcUnfreezeLocalFlag, "local", "", false, "Unfreeze inline the selected local instances.")
	svcUnfreezeCmd.Flags().BoolVarP(&svcUnfreezeWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func svcUnfreezeCmdRun(cmd *cobra.Command, args []string) {
	a := action.ObjectAction{
		ObjectSelector: mergeSelector(svcSelectorFlag, "svc", ""),
		NodeSelector:   svcUnfreezeNodeFlag,
		Local:          nodeUnfreezeLocalFlag,
		Action:         "freeze",
		Method:         "Unfreeze",
		Target:         "thawed",
		Watch:          svcUnfreezeWatchFlag,
		Format:         formatFlag,
		Color:          colorFlag,
	}
	action.Do(a)
}
