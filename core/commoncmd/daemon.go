package commoncmd

import "github.com/spf13/cobra"

func NewCmdDaemon() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "manage the daemon and its components",
	}
}

func NewCmdDaemonDNS() *cobra.Command {
	return &cobra.Command{
		Use:   "dns",
		Short: "manage the nameserver",
	}
}

func NewCmdDaemonHeartbeat() *cobra.Command {
	return &cobra.Command{
		Use:   "hb",
		Short: "manage heartbeats",
	}
}

func NewCmdDaemonListener() *cobra.Command {
	return &cobra.Command{
		Use:   "listener",
		Short: "manage listeners",
	}
}

func NewCmdDaemonRelay() *cobra.Command {
	return &cobra.Command{
		Use:   "relay",
		Short: "manage the relay server",
	}
}
