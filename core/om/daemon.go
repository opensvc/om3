package om

import (
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/util/hostname"
)

func init() {
	cmdDaemon := commoncmd.NewCmdDaemon()
	cmdDaemonDNS := commoncmd.NewCmdDaemonDNS()
	cmdDaemonHeartbeat := commoncmd.NewCmdDaemonHeartbeat()
	cmdDaemonListener := commoncmd.NewCmdDaemonListener()
	cmdDaemonRelay := commoncmd.NewCmdDaemonRelay()

	root.AddCommand(
		cmdDaemon,
	)

	cmdDaemon.AddCommand(
		cmdDaemonDNS,
		cmdDaemonHeartbeat,
		cmdDaemonListener,
		newCmdDaemonJoin(),
		newCmdDaemonLeave(),
		cmdDaemonRelay,
		newCmdDaemonRestart(),
		newCmdDaemonRun(),
		newCmdDaemonRunning(),
		newCmdDaemonShutdown(),
		newCmdDaemonStart(),
		newCmdDaemonStop(),
		commoncmd.NewCmdDaemonAuth(),
		commoncmd.NewCmdDaemonLog(),
		commoncmd.NewCmdDaemonStatus(),
	)

	cmdDaemonDNS.AddCommand(
		commoncmd.NewCmdDaemonDNSDump(),
	)

	cmdDaemonHeartbeat.AddCommand(
		commoncmd.NewCmdDaemonHeartbeatStatus(hostname.Hostname()),
		commoncmd.NewCmdDaemonHeartbeatRestart(),
		commoncmd.NewCmdDaemonHeartbeatStart(),
		commoncmd.NewCmdDaemonHeartbeatStop(),
	)

	cmdDaemonListener.AddCommand(
		commoncmd.NewCmdDaemonListenerRestart(),
		commoncmd.NewCmdDaemonListenerStart(),
		commoncmd.NewCmdDaemonListenerStop(),
		commoncmd.NewCmdDaemonListenerLog(),
	)

	cmdDaemonRelay.AddCommand(
		commoncmd.NewCmdDaemonRelayStatus(),
	)
}
