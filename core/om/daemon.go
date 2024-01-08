package om

import (
	"github.com/spf13/cobra"
)

var (
	cmdDaemon = &cobra.Command{
		Use:   "daemon",
		Short: "Manage the opensvc daemon",
	}

	cmdDaemonDNS = &cobra.Command{
		Use:   "dns",
		Short: "dns subsystem commands",
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
		cmdDaemonDNS,
		newCmdDaemonJoin(),
		newCmdDaemonLeave(),
		cmdDaemonRelay,
		newCmdDaemonRestart(),
		newCmdDaemonRunning(),
		newCmdDaemonShutdown(),
		newCmdDaemonStart(),
		newCmdDaemonStats(),
		newCmdDaemonStatus(),
		newCmdDaemonStop(),
	)
	cmdDaemonDNS.AddCommand(
		newCmdDaemonDNSDump(),
	)
	cmdDaemonRelay.AddCommand(
		newCmdDaemonRelayStatus(),
	)
}
