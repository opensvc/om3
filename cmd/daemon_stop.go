package cmd

import (
	"github.com/spf13/cobra"

	"opensvc.com/opensvc/daemon/daemoncli"
)

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop the daemon",
	RunE:  daemonStopCmdRun,
}

func init() {
	daemonCmd.AddCommand(daemonStopCmd)
}

func daemonStopCmdRun(_ *cobra.Command, _ []string) error {
	daemoncli.LockFuncExit("daemon stop", daemoncli.Stop)
	return nil
}
