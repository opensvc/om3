package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"opensvc.com/opensvc/daemon/daemoncli"
)

var daemonRunningCmd = &cobra.Command{
	Use:   "running",
	Short: "Return with code 0 if the daemon is running, else return with code 1",
	Run:   daemonRunningCmdRun,
}

func init() {
	daemonCmd.AddCommand(daemonRunningCmd)
}

func daemonRunningCmdRun(_ *cobra.Command, _ []string) {
	cli, err := newClient()
	if err != nil {
		os.Exit(1)
	}
	if daemoncli.New(cli).Running() {
		os.Exit(0)
	}
	os.Exit(1)
}
