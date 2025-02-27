package om

import (
	"github.com/spf13/cobra"
)

var (
	cmdDaemon = &cobra.Command{
		Use:   "daemon",
		Short: "Manage the opensvc daemon",
	}

	cmdDaemonComponent = &cobra.Command{
		Use:   "component",
		Short: "Manage opensvc daemon components",
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
		cmdDaemonComponent,
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

	cmdDaemonComponent.AddCommand(
		newCmdDaemonComponentAction("restart"),
		newCmdDaemonComponentAction("start"),
		newCmdDaemonComponentAction("stop"),
	)

	cmdDaemonDNS.AddCommand(
		newCmdDaemonDNSDump(),
	)

	cmdDaemonRelay.AddCommand(
		newCmdDaemonRelayStatus(),
	)
}
