package ox

import (
	"github.com/opensvc/om3/core/commoncmd"
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
	)
	cmdDaemon.AddCommand(
		cmdDaemonDNS,
		cmdDaemonHeartbeat,
		cmdDaemonListener,
		cmdDaemonRelay,
		newCmdDaemonRestart(),
		newCmdDaemonShutdown(),
		newCmdDaemonStop(),
		commoncmd.NewCmdDaemonAuth(),
		commoncmd.NewCmdDaemonLog(),
		commoncmd.NewCmdDaemonStatus(),
	)

	cmdDaemonDNS.AddCommand(
		commoncmd.NewCmdDaemonDNSDump(),
	)

	cmdDaemonHeartbeat.AddCommand(
		commoncmd.NewCmdDaemonHeartbeatRestart(),
		commoncmd.NewCmdDaemonHeartbeatStart(),
		commoncmd.NewCmdDaemonHeartbeatStop(),
		commoncmd.NewCmdDaemonHeartbeatStatus(""),
		commoncmd.NewCmdHeartbeatSign(),
		commoncmd.NewCmdHeartbeatWipe(),
		commoncmd.NewCmdDaemonHeartbeatRotate(),
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
