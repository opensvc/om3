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
	cli, err := newClient()
	if err != nil {
		return err
	}
	daemoncli.LockFuncExit("daemon stop", daemoncli.New(cli).Stop)
	return nil
}
