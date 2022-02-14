package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"opensvc.com/opensvc/daemon/daemoncli"
)

var daemonEventsCmd = &cobra.Command{
	Use:   "eventsdemo",
	Short: "Print the node event demo stream",
	Run:   daemonEventsCmdRun,
}

func init() {
	daemonCmd.AddCommand(daemonEventsCmd)
}

func daemonEventsCmdRun(_ *cobra.Command, _ []string) {
	cli, err := newClient()
	if err == nil && daemoncli.New(cli).Events() == nil {
		os.Exit(0)
	}
	os.Exit(1)
}
