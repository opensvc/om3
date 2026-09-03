package ox

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
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
		cmdDaemonRelay,
		newCmdDaemonEvents(),
		newCmdDaemonRestart(),
		newCmdDaemonShutdown(),
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
		commoncmd.NewCmdDaemonHeartbeatRestart(),
		commoncmd.NewCmdDaemonHeartbeatStart(),
		commoncmd.NewCmdDaemonHeartbeatStop(),
		commoncmd.NewCmdDaemonHeartbeatList(""),
		commoncmd.NewCmdDaemonHeartbeatStatus(""),
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
