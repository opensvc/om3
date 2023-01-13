package cmd

import (
	"github.com/spf13/cobra"
)

var (
	cmdDaemon = &cobra.Command{
		Use:   "daemon",
		Short: "Manage the opensvc daemon",
	}

	cmdDaemonRelay = &cobra.Command{
		Use:   "relay",
		Short: "relay subsystem commands",
	}
)

func init() {
	root.AddCommand(
		cmdDaemon,
	)
	cmdDaemon.AddCommand(
		newCmdDaemonAuth(),
		newCmdDaemonJoin(),
		cmdDaemonRelay,
		newCmdDaemonRestart(),
		newCmdDaemonRunning(),
		newCmdDaemonStart(),
		newCmdDaemonStats(),
		newCmdDaemonStatus(),
		newCmdDaemonStop(),
	)
	cmdDaemonRelay.AddCommand(
		newCmdDaemonRelayStatus(),
	)

}
