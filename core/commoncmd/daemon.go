package commoncmd

import "github.com/spf13/cobra"

func NewCmdDaemon() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "daemon",
		Short:   "manage the daemon and its components",
	}
}

func NewCmdDaemonDNS() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "dns",
		Short:   "manage the nameserver",
	}
}

func NewCmdDaemonHeartbeat() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "hb",
		Short:   "manage heartbeats",
	}
}

func NewCmdDaemonListener() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "listener",
		Short:   "manage listeners",
	}
}

func NewCmdDaemonRelay() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "relay",
		Short:   "manage the relay server",
	}
}

func NewCmdDaemonRestart() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "restart the daemon",
		Long:  "restart the daemon. Operation is asynchronous when node selector is used",
	}
}

func NewCmdDaemonShutdown() *cobra.Command {
	return &cobra.Command{
		Use:   "shutdown",
		Short: "shutdown all local svc and vol objects then shutdown the daemon",
	}
}

func NewCmdDaemonRun() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "run the daemon in foreground",
		Long:  "Start executes a detached run",
	}
}

func NewCmdDaemonRunning() *cobra.Command {
	return &cobra.Command{
		Use:   "running",
		Short: "test if the daemon is running",
		Long:  "Exit with code 0 if the daemon is running, else exit with code 1",
	}
}

func NewCmdDaemonStart() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "start the daemon or a daemon subsystem",
	}
}

func NewCmdDaemonStop() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "stop the daemon",
	}
}

// NewCmdDaemonSession returns the command group of the commands submitted to
// the daemon, which it remembers so a client that submitted one can ask how it
// ended.
func NewCmdDaemonSession() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "session",
		Short:   "submitted command commands",
	}
}

// NewCmdDaemonExec returns the command group of the runs the daemon made,
// one per object per node, which is the scale the outcome is at.
func NewCmdDaemonExec() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "exec",
		Short:   "command execution commands",
	}
}

// NewCmdDaemonOrchestration returns the command group of the orchestrations
// the monitor accepted.
func NewCmdDaemonOrchestration() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "orchestration",
		Short:   "orchestration commands",
		Aliases: []string{"orch"},
	}
}
