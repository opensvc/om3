package om

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/util/hostname"
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

	cmdDaemon.AddGroup(
		commoncmd.NewGroupQuery(),
		commoncmd.NewGroupSubsystems(),
	)
	cmdDaemon.AddCommand(
		cmdDaemonDNS,
		cmdDaemonHeartbeat,
		cmdDaemonListener,
		newCmdDaemonJoin(),
		newCmdDaemonLeave(),
		cmdDaemonRelay,
		newCmdDaemonEvents(),
		newCmdDaemonRestart(),
		newCmdDaemonRun(),
		newCmdDaemonRunning(),
		newCmdDaemonShutdown(),
		newCmdDaemonStart(),
		newCmdDaemonStop(),
		commoncmd.NewCmdDaemonAuth(),
		commoncmd.NewCmdDaemonAudit(),
		commoncmd.NewCmdDaemonLog(),
		commoncmd.NewCmdDaemonStatus(),
		commoncmd.NewCmdDaemonPs(),
		commoncmd.NewCmdDaemonKill(),
	)

	cmdDaemonDNS.AddCommand(
		commoncmd.NewCmdDaemonDNSDump(),
	)

	cmdDaemonHeartbeat.AddCommand(
		commoncmd.NewCmdDaemonHeartbeatList(hostname.Hostname()),
		commoncmd.NewCmdDaemonHeartbeatStatus(hostname.Hostname()),
		commoncmd.NewCmdDaemonHeartbeatRestart(),
		commoncmd.NewCmdDaemonHeartbeatStart(),
		commoncmd.NewCmdDaemonHeartbeatStop(),
		commoncmd.NewCmdHeartbeatSign(),
		commoncmd.NewCmdHeartbeatWipe(),
		commoncmd.NewCmdDaemonHeartbeatRotate(),
	)

	cmdDaemonListener.AddCommand(
		commoncmd.NewCmdDaemonListenerRestart(),
		commoncmd.NewCmdDaemonListenerStart(),
		commoncmd.NewCmdDaemonListenerStop(),
	)

	cmdDaemonRelay.AddCommand(
		commoncmd.NewCmdDaemonRelayList(),
		commoncmd.NewCmdDaemonRelayStatus(),
	)
}
