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
	daemonRunningCmd.Flags().StringVarP(&nodeFlag, "node", "", "", "the nodes to execute the action on")
}

func daemonRunningCmdRun(_ *cobra.Command, _ []string) {
	cli, err := newClient()
	if err != nil {
		os.Exit(1)
	}
	dCli := daemoncli.New(cli)
	if nodeFlag == "" {
		dCli.SetNode(nodeFlag)
	} else {
		dCli.SetNode(nodeFlag)
	}
	if dCli.Running() {
		os.Exit(0)
	}
	os.Exit(1)
}
