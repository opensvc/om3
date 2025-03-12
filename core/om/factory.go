package om

import (
	// Necessary to use go:embed
	_ "embed"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/monitor"
	commands "github.com/opensvc/om3/core/omcmd"
)

var (
	//go:embed text/node-events/event-kind
	eventKindTemplate string
)

func newCmdAll() *cobra.Command {
	return &cobra.Command{
		Use:   "all",
		Short: "manage a mix of objects, tentatively exposing all commands",
	}
}

func newCmdCcfg() *cobra.Command {
	return &cobra.Command{
		Use:   "ccfg",
		Short: "manage the cluster shared configuration",
		Long: `The cluster nodes merge their private configuration
over the cluster shared configuration.

The shared configuration is hosted in a ccfg-kind object, and is
replicated using the same rules as other kinds of object (last write is
eventually replicated).
`,
	}
}

func newCmdCfg() *cobra.Command {
	return &cobra.Command{
		Use:   "cfg",
		Short: "manage configmaps",
		Long: `A configmap is an unencrypted key-value store.

Values can be binary or text.

A key can be installed as a file in a Vol, then exposed to apps
and containers.

A key can be exposed as a environment variable for apps and
containers.

A signal can be sent to consumer processes upon exposed key value
changes.

The key names can include the '/' character, interpreted as a path separator
when installing the key in a volume.`,
	}
}

func newCmdSec() *cobra.Command {
	return &cobra.Command{
		Use:   "sec",
		Short: "manage secrets",
		Long: `A secret is an encrypted key-value store.

Values can be binary or text.

A key can be installed as a file in a Vol, then exposed to apps
and containers.

A key can be exposed as a environment variable for apps and
containers.

A signal can be sent to consumer processes upon exposed key value
changes.

The key names can include the '/' character, interpreted as a path separator
when installing the key in a volume.`,
	}
}

func newCmdSVC() *cobra.Command {
	return &cobra.Command{
		Use:   "svc",
		Short: "manage services",
		Long: `Service objects subsystem.
	
A service is typically made of ip, app, container and task resources.

They can use support objects like volumes, secrets and configmaps to
isolate lifecycles or to abstract cluster-specific knowledge.
`,
	}
}

func newCmdVol() *cobra.Command {
	return &cobra.Command{
		Use:   "vol",
		Short: "manage volumes",
		Long: `A volume is a persistent data provider.

A volume is made of disk, fs and sync resources. It is created by a pool,
to satisfy a demand from a volume resource in a service.

Volumes and their subdirectories can be mounted inside containers.

A volume can host cfg and sec keys projections.`,
	}
}

func newCmdUsr() *cobra.Command {
	return &cobra.Command{
		Use:   "usr",
		Short: "manage users",
		Long: `A user stores the grants and credentials of user of the agent API.

User objects are not necessary with OpenID authentication, as the
grants are embedded in the trusted bearer tokens.`,
	}
}

func newCmdArrayList() *cobra.Command {
	var options commands.CmdArrayList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the cluster-managed storage arrays",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdClusterAbort() *cobra.Command {
	var options commands.CmdClusterAbort
	cmd := &cobra.Command{
		Use:   "abort",
		Short: "abort the running orchestration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdClusterFreeze() *cobra.Command {
	var options commands.CmdClusterFreeze
	cmd := &cobra.Command{
		Use:   "freeze",
		Short: "block ha automatic start and split action on all nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	return cmd
}

func newCmdClusterLogs() *cobra.Command {
	var options commands.CmdClusterLogs
	cmd := &cobra.Command{
		Use:     "logs",
		Aliases: []string{"logs", "log", "lo"},
		Short:   "show all nodes logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLogs(flags, &options.OptsLogs)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

// newCmdClusterThaw creates a hidden 'thaw' subcommand alias for 'unfreeze' to unblock HA automatic and split actions.
func newCmdClusterThaw() *cobra.Command {
	cmd := newCmdClusterUnfreeze()
	cmd.Use = "thaw"
	cmd.Hidden = true
	return cmd
}

func newCmdClusterUnfreeze() *cobra.Command {
	var options commands.CmdClusterUnfreeze
	cmd := &cobra.Command{
		Use:    "unfreeze",
		Hidden: false,
		Short:  "unblock ha automatic and split action start on all nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	return cmd
}

func newCmdDaemonAuth() *cobra.Command {
	var options commands.CmdDaemonAuth
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "create new token",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagRoles(flags, &options.Roles)
	flags.DurationVar(&options.Duration, "duration", 60*time.Second, "token duration.")
	flags.StringSliceVar(&options.Out, "out", []string{"token"}, "the fields to display: [token,expired_at]")
	flags.StringVar(&options.Subject, "subject", "", "the subject of the token")
	flags.StringVar(&options.Scope, "scope", "", "the scope of the token grant")

	return cmd
}

func newCmdDaemonHeartbeatRestart() *cobra.Command {
	options := commands.CmdDaemonHeartbeatRestart{}
	cmd := &cobra.Command{
		Use:   "restart",
		Short: fmt.Sprintf("restart daemon heartbeat component `name`"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagDaemonHeartbeatName(flags, &options.Name)
	cmd.MarkFlagRequired("name")
	return cmd
}

func newCmdDaemonHeartbeatStart() *cobra.Command {
	options := commands.CmdDaemonHeartbeatStart{}
	cmd := &cobra.Command{
		Use:   "start",
		Short: fmt.Sprintf("start daemon heartbeat component `name`"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagDaemonHeartbeatName(flags, &options.Name)
	cmd.MarkFlagRequired("name")
	return cmd
}

func newCmdDaemonHeartbeatStop() *cobra.Command {
	options := commands.CmdDaemonHeartbeatStop{}
	cmd := &cobra.Command{
		Use:   "stop",
		Short: fmt.Sprintf("stop daemon heartbeat component `name`"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagDaemonHeartbeatName(flags, &options.Name)
	cmd.MarkFlagRequired("name")
	return cmd
}

func newCmdDaemonDNSDump() *cobra.Command {
	var options commands.CmdDNSDump
	cmd := &cobra.Command{
		Use:   "dump",
		Short: "dump the content of the cluster zone",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdDaemonJoin() *cobra.Command {
	var options commands.CmdDaemonJoin
	cmd := &cobra.Command{
		Use:   "join",
		Short: "add this node to a cluster",
		Long: "Join the cluster of the node specified by '--node <node>'.\n" +
			"The remote node expects the joiner to provide a join token using '--token <base64>'.\n" +
			"The join token can be created on the remote node by the 'daemon auth token --role join' command or by getting /auth/token with a user having the joiner or root role.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&options.Node, "node", "", "the name of the cluster node we want to join")

	if err := cmd.MarkFlagRequired("node"); err != nil {
		panic(err)
	}
	flags.StringVar(&options.Token, "token", "", "auth token with 'join' role"+
		" (created from 'om daemon auth --role join')")
	if err := cmd.MarkFlagRequired("token"); err != nil {
		panic(err)
	}
	flags.DurationVar(&options.Timeout, "timeout", 5*time.Second, "maximum duration to wait for local node added to cluster")
	return cmd
}

func newCmdDaemonLeave() *cobra.Command {
	var options commands.CmdDaemonLeave
	cmd := &cobra.Command{
		Use:   "leave",
		Short: "remove this node from a cluster",
		Long:  "Inform peer nodes we leave the cluster. Make sure the leaving node is no longer in the objects nodes list.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	flags.DurationVar(&options.Timeout, "timeout", 0, "maximum duration to wait for local node removed from cluster")
	return cmd
}

func newCmdDaemonListenerRestart() *cobra.Command {
	options := commands.CmdDaemonListenerRestart{}
	cmd := &cobra.Command{
		Use:   "restart",
		Short: fmt.Sprintf("restart daemon listener component `name`"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagDaemonListenerName(flags, &options.Name)
	cmd.MarkFlagRequired("name")
	return cmd
}

func newCmdDaemonListenerStart() *cobra.Command {
	options := commands.CmdDaemonListenerStart{}
	cmd := &cobra.Command{
		Use:   "start",
		Short: fmt.Sprintf("start daemon listener component `name`"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagDaemonListenerName(flags, &options.Name)
	cmd.MarkFlagRequired("name")
	return cmd
}

func newCmdDaemonListenerStop() *cobra.Command {
	options := commands.CmdDaemonListenerStop{}
	cmd := &cobra.Command{
		Use:   "stop",
		Short: fmt.Sprintf("stop daemon listener component `name`"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagDaemonListenerName(flags, &options.Name)
	cmd.MarkFlagRequired("name")
	return cmd
}

func newCmdDaemonRelayStatus() *cobra.Command {
	var options commands.CmdDaemonRelayStatus
	cmd := &cobra.Command{
		Use:   "status",
		Short: "show the local daemon relay clients and last data update time",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdDaemonRestart() *cobra.Command {
	var options commands.CmdDaemonRestart
	cmd := &cobra.Command{
		Use:     "restart",
		Short:   "restart the daemon",
		Long:    "restart the daemon. Operation is asynchronous when node selector is used",
		Aliases: []string{"restart"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagCPUProfile(flags, &options.CPUProfile)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdDaemonRun() *cobra.Command {
	var options commands.CmdDaemonRun
	cmd := &cobra.Command{
		Use:     "run",
		Short:   "run the daemon in foreground",
		Long:    "Start executes a detached run",
		Aliases: []string{"star"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagCPUProfile(flags, &options.CPUProfile)
	return cmd
}

func newCmdDaemonRunning() *cobra.Command {
	var options commands.CmdDaemonRunning
	cmd := &cobra.Command{
		Use:   "running",
		Short: "test if the daemon is running",
		Long:  "Exit with code 0 if the daemon is running, else exit with code 1",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdDaemonShutdown() *cobra.Command {
	var options commands.CmdDaemonShutdown
	cmd := &cobra.Command{
		Use:   "shutdown",
		Short: "shutdown all local svc and vol objects then shutdown the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagDuration(flags, &options.Timeout)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdDaemonStart() *cobra.Command {
	var options commands.CmdDaemonStart
	cmd := &cobra.Command{
		Use:     "start",
		Short:   "start the daemon or a daemon subsystem",
		Aliases: []string{"star"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagCPUProfile(flags, &options.CPUProfile)
	return cmd
}

func newCmdDaemonStatus() *cobra.Command {
	var options commands.CmdObjectMonitor
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "show the cluster status",
		Long:    monitor.CmdLong,
		Aliases: []string{"statu"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run("**", "")
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagWatch(flags, &options.Watch)
	commoncmd.FlagOutputSections(flags, &options.Sections)
	return cmd
}

func newCmdDaemonStop() *cobra.Command {
	var options commands.CmdDaemonStop
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "stop the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdKeystoreAdd(kind string) *cobra.Command {
	var options commands.CmdKeystoreAdd
	var from, value string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "add new keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flag("from").Changed {
				options.From = &from
			}
			if cmd.Flag("value").Changed {
				options.Value = &value
			}
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagKey(flags, &options.Key)
	commoncmd.FlagFrom(flags, &from)
	commoncmd.FlagValue(flags, &value)
	cmd.MarkFlagsMutuallyExclusive("from", "value")
	return cmd
}

func newCmdKeystoreChange(kind string) *cobra.Command {
	var options commands.CmdKeystoreChange
	var from, value string
	cmd := &cobra.Command{
		Use:   "change",
		Short: "change existing keys value",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flag("from").Changed {
				options.From = &from
			}
			if cmd.Flag("value").Changed {
				options.Value = &value
			}
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagKey(flags, &options.Key)
	commoncmd.FlagFrom(flags, &from)
	commoncmd.FlagValue(flags, &value)
	cmd.MarkFlagsMutuallyExclusive("from", "value")
	return cmd
}

func newCmdKeystoreDecode(kind string) *cobra.Command {
	var options commands.CmdKeystoreDecode
	cmd := &cobra.Command{
		Use:   "decode",
		Short: "decode a key value",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagKey(flags, &options.Key)
	return cmd
}

func newCmdKeystoreInstall(kind string) *cobra.Command {
	var options commands.CmdKeystoreInstall
	cmd := &cobra.Command{
		Use:   "install",
		Short: "install keys as files in volumes",
		Long:  "Keys of sec and cfg can be projected to volumes via the configs and secrets keywords of volume resources. When a key value change all projections are automatically refreshed. This command triggers manually the same operations.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagKey(flags, &options.Key)
	return cmd
}

func newCmdKeystoreKeys(kind string) *cobra.Command {
	var options commands.CmdKeystoreKeys
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "list the keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagMatch(flags, &options.Match)
	return cmd
}

func newCmdKeystoreRemove(kind string) *cobra.Command {
	var options commands.CmdKeystoreRemove
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "remove a key",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagKey(flags, &options.Key)
	return cmd
}

func newCmdKeystoreRename(kind string) *cobra.Command {
	var options commands.CmdKeystoreRename
	cmd := &cobra.Command{
		Use:   "rename",
		Short: "rename a key",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagKey(flags, &options.Key)
	commoncmd.FlagKeyTo(flags, &options.To)
	return cmd
}

func newCmdMonitor() *cobra.Command {
	var options commands.CmdObjectMonitor
	cmd := &cobra.Command{
		Use:     "monitor",
		Aliases: []string{"m", "mo", "mon", "moni", "monit", "monito"},
		Short:   "show the cluster status",
		Long:    monitor.CmdLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run("*", "")
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagWatch(flags, &options.Watch)
	commoncmd.FlagOutputSections(flags, &options.Sections)
	return cmd
}

func newCmdNetworkList() *cobra.Command {
	var options commands.CmdNetworkList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the cluster networks",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdNetworkSetup() *cobra.Command {
	var options commands.CmdNetworkSetup
	cmd := &cobra.Command{
		Use:     "setup",
		Short:   "configure the cluster networks on the node",
		Long:    "Most cluster network drivers need ip routes, ip rules, tunnels and firewall rules. This command sets them up, the same as done on daemon startup and daemon reconfiguration via configuration change.",
		Aliases: []string{"set"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdNetworkIPList() *cobra.Command {
	var options commands.CmdNetworkIPList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the ip in the cluster networks",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNetworkStatusName(flags, &options.Name)
	return cmd
}

func newCmdNodeAbort() *cobra.Command {
	var options commands.CmdNodeAbort
	cmd := &cobra.Command{
		Use:   "abort",
		Short: "abort the running orchestration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeCapabilitiesList() *cobra.Command {
	var options commands.CmdNodeCapabilitiesList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the node capabilities",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeCapabilitiesScan() *cobra.Command {
	var options commands.CmdNodeCapabilitiesScan
	cmd := &cobra.Command{
		Use:     "scan",
		Short:   "scan the node capabilities",
		Aliases: []string{"sca", "sc"},
		Long: `Scan the node for capabilities.

Capabilities are normally scanned at daemon startup and when the installed 
system packages change, so admins only have to use this when they want manually 
installed software to be discovered without restarting the daemon.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeChecks() *cobra.Command {
	var options commands.CmdNodeChecks
	cmd := &cobra.Command{
		Use:     "checks",
		Short:   "run the checks, push and print the result",
		Aliases: []string{"check", "chec", "che", "ch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeClear() *cobra.Command {
	var options commands.CmdNodeClear
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "reset the monitor state to idle",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdNodeCollectorTagAttach() *cobra.Command {
	var attachData string
	var options commands.CmdNodeCollectorTagAttach
	cmd := &cobra.Command{
		Use:     "attach",
		Short:   "attach a tag to this node",
		Long:    "The tag must already exist in the collector.",
		Aliases: []string{"atta", "att", "at", "a"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flag("attach-data").Changed {
				options.AttachData = &attachData
			}
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	flags.StringVar(&options.Name, "name", "", "the tag name")
	flags.StringVar(&attachData, "attach-data", "", "the data stored with the tag attachment")
	return cmd
}

func newCmdNodeCollectorTagCreate() *cobra.Command {
	var (
		data    string
		exclude string
	)
	var options commands.CmdNodeCollectorTagCreate
	cmd := &cobra.Command{
		Use:   "create",
		Short: "create a new tag",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flag("data").Changed {
				options.Data = &data
			}
			if cmd.Flag("exclude").Changed {
				options.Exclude = &exclude
			}
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	flags.StringVar(&options.Name, "name", "", "the tag name")
	flags.StringVar(&data, "data", "", "the data stored with the tag")
	flags.StringVar(&exclude, "exclude", "", "a pattern to prevent attachment of incompatible tags")
	return cmd
}

func newCmdNodeCollectorTagDetach() *cobra.Command {
	var options commands.CmdNodeCollectorTagDetach
	cmd := &cobra.Command{
		Use:     "detach",
		Short:   "detach a tag from this node",
		Aliases: []string{"deta", "det", "de", "d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	flags.StringVar(&options.Name, "name", "", "the tag name")
	return cmd
}

func newCmdNodeCollectorTagList() *cobra.Command {
	var options commands.CmdNodeCollectorTagList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list available tags",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdNodeCollectorTagShow() *cobra.Command {
	var options commands.CmdNodeCollectorTagShow
	cmd := &cobra.Command{
		Use:   "show",
		Short: "show tags attached to this node",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	flags.BoolVar(&options.Verbose, "verbose", false, "also show the attach data")
	return cmd
}

func newCmdNodeComplianceAttachModuleset() *cobra.Command {
	var options commands.CmdNodeComplianceAttachModuleset
	cmd := &cobra.Command{
		Use:     "moduleset",
		Short:   "attach modulesets to this node",
		Long:    "Modules of all attached modulesets are checked on schedule.",
		Aliases: []string{"modulese", "modules", "module", "modul", "modu", "mod", "mo"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeComplianceAttachRuleset() *cobra.Command {
	var options commands.CmdNodeComplianceAttachRuleset
	cmd := &cobra.Command{
		Use:     "ruleset",
		Short:   "attach rulesets to this node",
		Long:    "Rules of attached rulesets are exposed to modules.",
		Aliases: []string{"rulese", "rules", "rule", "rul", "ru"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagRuleset(flags, &options.Ruleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeComplianceAuto() *cobra.Command {
	var options commands.CmdNodeComplianceAuto
	cmd := &cobra.Command{
		Use:   "auto",
		Short: "run modules fixes or checks",
		Long:  "If the module is has the 'autofix' property set, do a fix, else do a check.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModule(flags, &options.Module)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagComplianceAttach(flags, &options.Attach)
	commoncmd.FlagComplianceForce(flags, &options.Force)
	return cmd
}

func newCmdNodeComplianceCheck() *cobra.Command {
	var options commands.CmdNodeComplianceCheck
	cmd := &cobra.Command{
		Use:     "check",
		Short:   "run modules checks",
		Aliases: []string{"chec", "che", "ch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModule(flags, &options.Module)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagComplianceAttach(flags, &options.Attach)
	commoncmd.FlagComplianceForce(flags, &options.Force)
	return cmd
}

func newCmdNodeComplianceFix() *cobra.Command {
	var options commands.CmdNodeComplianceFix
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "run modules fixes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModule(flags, &options.Module)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagComplianceAttach(flags, &options.Attach)
	commoncmd.FlagComplianceForce(flags, &options.Force)
	return cmd
}

func newCmdNodeComplianceFixable() *cobra.Command {
	var options commands.CmdNodeComplianceFixable
	cmd := &cobra.Command{
		Use:     "fixable",
		Short:   "run modules fixable-tests",
		Aliases: []string{"fixabl", "fixab", "fixa"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModule(flags, &options.Module)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagComplianceAttach(flags, &options.Attach)
	commoncmd.FlagComplianceForce(flags, &options.Force)
	return cmd
}

func newCmdNodeComplianceDetachModuleset() *cobra.Command {
	var options commands.CmdNodeComplianceDetachModuleset
	cmd := &cobra.Command{
		Use:     "moduleset",
		Short:   "detach modulesets from this node",
		Long:    "Modules of attached modulesets are checked on schedule.",
		Aliases: []string{"modulese", "modules", "module", "modul", "modu", "mod", "mo"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeComplianceDetachRuleset() *cobra.Command {
	var options commands.CmdNodeComplianceDetachRuleset
	cmd := &cobra.Command{
		Use:     "ruleset",
		Short:   "detach rulesets from this node",
		Long:    "Rules of attached rulesets are made available to their module.",
		Aliases: []string{"rulese", "rules", "rule", "rul", "ru"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagRuleset(flags, &options.Ruleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeComplianceEnv() *cobra.Command {
	var options commands.CmdNodeComplianceEnv
	cmd := &cobra.Command{
		Use:     "env",
		Short:   "show the env variables set for modules run",
		Aliases: []string{"en"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagModule(flags, &options.Module)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeComplianceListModules() *cobra.Command {
	var options commands.CmdNodeComplianceListModules
	cmd := &cobra.Command{
		Use:     "modules",
		Short:   "list modules available on this node",
		Aliases: []string{"module", "modul", "modu", "mod"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeComplianceListModuleset() *cobra.Command {
	var options commands.CmdNodeComplianceListModuleset
	cmd := &cobra.Command{
		Use:     "moduleset",
		Short:   "list modulesets available to this node",
		Aliases: []string{"modulesets"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeComplianceListRuleset() *cobra.Command {
	var options commands.CmdNodeComplianceListRuleset
	cmd := &cobra.Command{
		Use:     "ruleset",
		Short:   "list rulesets available to this node",
		Aliases: []string{"rulese", "rules", "rule", "rul", "ru"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagRuleset(flags, &options.Ruleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeComplianceShowModuleset() *cobra.Command {
	var options commands.CmdNodeComplianceShowModuleset
	cmd := &cobra.Command{
		Use:     "moduleset",
		Short:   "show modulesets and modules attached to this node",
		Aliases: []string{"modulese", "modules", "module", "modul", "modu", "mod", "mo"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeComplianceShowRuleset() *cobra.Command {
	var options commands.CmdNodeComplianceShowRuleset
	cmd := &cobra.Command{
		Use:     "ruleset",
		Short:   "show rules contextualized for this node",
		Aliases: []string{"rulese", "rules", "rule", "rul", "ru"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeDoc() *cobra.Command {
	var options commands.CmdNodeDoc
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "print the documentation of the selected keywords",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobalColor(flags, &options.OptsGlobal)
	addFlagsGlobalOutput(flags, &options.OptsGlobal)
	commoncmd.FlagKeyword(flags, &options.Keyword)
	commoncmd.FlagDriver(flags, &options.Driver)
	commoncmd.FlagDepth(flags, &options.Depth)
	cmd.MarkFlagsMutuallyExclusive("driver", "kw")
	return cmd
}

func newCmdNodeDrain() *cobra.Command {
	var options commands.CmdNodeDrain
	cmd := &cobra.Command{
		Use:   "drain",
		Short: "freeze node and shutdown all its object instances",
		Long:  "If not specified with --node, the local node is selected for drain.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeDrivers() *cobra.Command {
	var options commands.CmdNodeDrivers
	cmd := &cobra.Command{
		Use:     "drivers",
		Short:   "list builtin drivers",
		Aliases: []string{"driver", "drive", "driv", "drv", "dr"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobalColor(flags, &options.OptsGlobal)
	addFlagsGlobalOutput(flags, &options.OptsGlobal)
	return cmd
}

func newCmdNodeEdit() *cobra.Command {
	var options commands.CmdNodeConfigEdit
	cmd := &cobra.Command{
		Use:     "edit",
		Short:   "edit the node configuration",
		Hidden:  true,
		Aliases: []string{"ed", "edi"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagDiscard(flags, &options.Discard)
	commoncmd.FlagRecover(flags, &options.Recover)
	cmd.MarkFlagsMutuallyExclusive("discard", "recover")
	return cmd
}

func newCmdNodeConfigEdit() *cobra.Command {
	var options commands.CmdNodeConfigEdit
	cmd := &cobra.Command{
		Use:     "edit",
		Short:   "edit the node configuration",
		Aliases: []string{"ed"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagDiscard(flags, &options.Discard)
	commoncmd.FlagRecover(flags, &options.Recover)
	cmd.MarkFlagsMutuallyExclusive("discard", "recover")
	return cmd
}

func newCmdNodeEditConfig() *cobra.Command {
	var options commands.CmdNodeConfigEdit
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "edit the node configuration",
		Aliases: []string{"conf", "co", "cf", "cfg"},
		Hidden:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagDiscard(flags, &options.Discard)
	commoncmd.FlagRecover(flags, &options.Recover)
	cmd.MarkFlagsMutuallyExclusive("discard", "recover")
	return cmd
}

func newCmdNodeConfigEval() *cobra.Command {
	var options commands.CmdNodeConfigEval
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "evaluate a configuration key value",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagImpersonate(flags, &options.Impersonate)
	commoncmd.FlagKeywords(flags, &options.Keywords)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	cmd.MarkFlagRequired("kw")
	return cmd
}

func newCmdNodeEvents() *cobra.Command {
	var options commands.CmdNodeEvents
	cmd := &cobra.Command{
		Use:     "events",
		Short:   "print the node event stream",
		Long:    "Print the node event stream\n\nAvailable kinds: \n" + eventKindTemplate,
		Aliases: []string{"eve", "even", "event"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagEventFilters(flags, &options.Filters)
	commoncmd.FlagDuration(flags, &options.Duration)
	commoncmd.FlagEventTemplate(flags, &options.Template)
	commoncmd.FlagWait(flags, &options.Wait)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	flags.Uint64Var(&options.Limit, "limit", 0, "stop listening when <limit> events are received, the default is 0 (unlimited) or 1 if --wait is set")
	return cmd
}

func newCmdNodeFreeze() *cobra.Command {
	var options commands.CmdNodeFreeze
	cmd := &cobra.Command{
		Use:   "freeze",
		Short: "block ha automatic start and split action",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeConfigGet() *cobra.Command {
	var options commands.CmdNodeConfigGet
	cmd := &cobra.Command{
		Use:   "get",
		Short: "get a configuration key value",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagEval(flags, &options.Eval)
	commoncmd.FlagImpersonate(flags, &options.Impersonate)
	commoncmd.FlagKeywords(flags, &options.Keywords)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeLogs() *cobra.Command {
	var options commands.CmdNodeLogs
	cmd := &cobra.Command{
		Use:     "logs",
		Aliases: []string{"logs", "log", "lo"},
		Short:   "show this node logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLogs(flags, &options.OptsLogs)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeList() *cobra.Command {
	var options commands.CmdNodeList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the cluster nodes",
		Long:    "The list can be filtered using the --node selector. This command can be used to validate node selector expressions.",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodePRKey() *cobra.Command {
	var options commands.CmdNodePRKey
	cmd := &cobra.Command{
		Use:     "prkey",
		Short:   "show the scsi3 persistent reservation key of this node",
		Aliases: []string{"prk", "prke"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeConfigShow() *cobra.Command {
	var options commands.CmdNodeConfigShow
	cmd := &cobra.Command{
		Use:   "show",
		Short: "show the node configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagEval(flags, &options.Eval)
	commoncmd.FlagImpersonate(flags, &options.Impersonate)
	return cmd
}

func newCmdObjectPrintResourceInfo(kind string) *cobra.Command {
	var options commands.CmdObjectResourceInfoList
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "resinfo",
		Short:  "list the key-values reported by the resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeScheduleList() *cobra.Command {
	var options commands.CmdNodeScheduleList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the node scheduler entries",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodePushasset() *cobra.Command {
	var options commands.CmdNodePushAsset
	cmd := &cobra.Command{
		Use:     "pushasset",
		Hidden:  true,
		Short:   "run the node discovery, push and print the result",
		Aliases: []string{"pushasse", "pushass", "pushas"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodePushAsset() *cobra.Command {
	var options commands.CmdNodePushAsset
	cmd := &cobra.Command{
		Use:     "asset",
		Short:   "run the node discovery, push and print the result",
		Aliases: []string{"asse", "ass", "as"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodePushdisk() *cobra.Command {
	var options commands.CmdNodePushDisks
	cmd := &cobra.Command{
		Use:     "pushdisk",
		Hidden:  true,
		Short:   "run the disk discovery, push and print the result",
		Aliases: []string{"pushdisks", "pushdis", "psuhdi"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodePushDisk() *cobra.Command {
	var options commands.CmdNodePushDisks
	cmd := &cobra.Command{
		Use:     "disk",
		Short:   "run the disk discovery, push and print the result",
		Aliases: []string{"disks", "dis", "di"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodePushpatch() *cobra.Command {
	var options commands.CmdNodePushPatch
	cmd := &cobra.Command{
		Use:     "pushpatch",
		Hidden:  true,
		Short:   "run the node installed patches discovery, push and print the result",
		Aliases: []string{"pushpatc", "pushpat", "pushpa"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodePushPatch() *cobra.Command {
	var options commands.CmdNodePushPatch
	cmd := &cobra.Command{
		Use:     "patch",
		Short:   "run the node installed patches discovery, push and print the result",
		Aliases: []string{"patc", "pat", "pa"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodePushpkg() *cobra.Command {
	var options commands.CmdNodePushPkg
	cmd := &cobra.Command{
		Use:     "pushpkg",
		Hidden:  true,
		Short:   "run the node installed packages discovery, push and print the result",
		Aliases: []string{"pushpk"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodePushPkg() *cobra.Command {
	var options commands.CmdNodePushPkg
	cmd := &cobra.Command{
		Use:     "pkg",
		Short:   "run the node installed packages discovery, push and print the result",
		Aliases: []string{"pk"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeRegister() *cobra.Command {
	var options commands.CmdNodeRegister
	cmd := &cobra.Command{
		Use:     "register",
		Short:   "initial login on the collector",
		Long:    "Obtain a registration id from the collector, store it in the node configuration node.uuid keyword. This uuid is then used to authenticate the node in collector communications.",
		Aliases: []string{"registe", "regist", "regis", "regi", "reg", "re"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagCollectorUser(flags, &options.User)
	commoncmd.FlagCollectorPassword(flags, &options.Password)
	commoncmd.FlagCollectorApp(flags, &options.App)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)

	return cmd
}

func newCmdNodeRelayStatus() *cobra.Command {
	var options commands.CmdNodeRelayStatus
	cmd := &cobra.Command{
		Use:   "status",
		Short: "show the clients and last data update time of the configured relays",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flagSet := cmd.Flags()
	addFlagsGlobal(flagSet, &options.OptsGlobal)
	commoncmd.FlagRelay(flagSet, &options.Relays)
	return cmd
}

func newCmdNodeConfigUpdate() *cobra.Command {
	var options commands.CmdNodeConfigUpdate
	cmd := &cobra.Command{
		Use:   "update",
		Short: "update the node configuration",
		Long:  "Apply section deletes, keyword unsets then sets. Validate the new configuration and commit.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagUpdateDelete(flags, &options.Delete)
	commoncmd.FlagUpdateSet(flags, &options.Set)
	commoncmd.FlagUpdateUnset(flags, &options.Unset)
	return cmd
}

func newCmdNodeSysreport() *cobra.Command {
	var options commands.CmdNodeSysreport
	cmd := &cobra.Command{
		Use:     "sysreport",
		Short:   "collect system data and push it to the collector",
		Long:    "Push system report to the collector for archiving and diff analysis. The --force option resend all monitored files and outputs to the collector instead of only those that changed since the last sysreport.",
		Aliases: []string{"sysrepor", "sysrepo", "sysrep", "sysre", "sysr", "sys", "sy"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagForce(flags, &options.Force)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeUnfreeze() *cobra.Command {
	var options commands.CmdNodeUnfreeze
	cmd := &cobra.Command{
		Use:     "unfreeze",
		Short:   "unblock ha automatic start and split action",
		Aliases: []string{"thaw"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeConfigValidate() *cobra.Command {
	var options commands.CmdNodeConfigValidate
	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "verify the node configuration syntax",
		Aliases: []string{"val", "valid"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdNodeValidateConfig() *cobra.Command {
	var options commands.CmdNodeConfigValidate
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "verify the node configuration syntax",
		Aliases: []string{"conf", "co", "cf", "cfg"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectAbort(kind string) *cobra.Command {
	var options commands.CmdObjectAbort
	cmd := &cobra.Command{
		Use:   "abort",
		Short: "abort the running orchestration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdObjectBoot(kind string) *cobra.Command {
	var options commands.CmdObjectBoot
	cmd := &cobra.Command{
		Use:    "boot",
		Hidden: true,
		Short:  "clean up actions executed on boot only",
		Long:   "SCSI reservation release, vg tags removal, ... Never execute this action manually.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeSSHTrust() *cobra.Command {
	var options commands.CmdNodeSSHTrust
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "ssh-trust node peers",
		Long: "Configure the nodes specified by the --node flag to allow SSH communication from their peers." +
			" By default, the trusted SSH key is opensvc, but this can be customized using the node.sshkey setting." +
			" If the key does not exist, OpenSVC automatically generates it.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdNodeUpdateSSHKeys() *cobra.Command {
	cmd := newCmdNodeSSHTrust()
	cmd.Use = "keys"
	program := os.Args[0]
	cmd.Deprecated = fmt.Sprintf("use the \"%s node ssh trust\" or \"%s cluster ssh trust\" command instead.", program, program)
	return cmd
}

func newCmdClusterSSHTrust() *cobra.Command {
	var options commands.CmdClusterSSHTrust
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "ssh-trust all the node mesh",
		Long: "Configure all nodes to allow SSH communication from their peers." +
			" By default, the trusted SSH key is opensvc, but this can be customized using the node.sshkey setting." +
			" If the key does not exist, OpenSVC automatically generates it.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdObjectClear(kind string) *cobra.Command {
	var options commands.CmdObjectClear
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "clear errors in the monitor state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdObjectPrint(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "print",
		Short:   "print information about the object",
		Aliases: []string{"prin", "pri", "pr"},
	}
}

func newCmdObjectPush(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "push",
		Short:   "push information about the object to the collector",
		Aliases: []string{"push", "pus", "pu"},
	}
}

func newCmdObjectSSH(kind string) *cobra.Command {
	return &cobra.Command{
		Use:   "ssh",
		Short: "ssh command group",
	}
}

func newCmdObjectSync(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "sync",
		Short:   "data synchronization command group",
		Aliases: []string{"syn", "sy"},
	}
}

func newCmdObjectCollector(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "collector",
		Short:   "collector data management commands",
		Aliases: []string{"coll"},
	}
}

func newCmdObjectCollectorTag(kind string) *cobra.Command {
	return &cobra.Command{
		Use:   "tag",
		Short: "collector tags management commands",
	}
}

func newCmdObjectCollectorTagAttach(kind string) *cobra.Command {
	var attachData string
	var options commands.CmdObjectCollectorTagAttach
	cmd := &cobra.Command{
		Use:     "attach",
		Short:   "attach a tag to this node",
		Long:    "The tag must already exist in the collector.",
		Aliases: []string{"atta", "att", "at", "a"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flag("attach-data").Changed {
				options.AttachData = &attachData
			}
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	flags.StringVar(&options.Name, "name", "", "the tag name")
	flags.StringVar(&attachData, "attach-data", "", "the data stored with the tag attachment")
	return cmd
}

func newCmdObjectCollectorTagCreate(kind string) *cobra.Command {
	var (
		data    string
		exclude string
	)
	var options commands.CmdObjectCollectorTagCreate
	cmd := &cobra.Command{
		Use:   "create",
		Short: "create a new tag",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flag("data").Changed {
				options.Data = &data
			}
			if cmd.Flag("exclude").Changed {
				options.Exclude = &exclude
			}
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	flags.StringVar(&options.Name, "name", "", "the tag name")
	flags.StringVar(&data, "data", "", "the data stored with the tag")
	flags.StringVar(&exclude, "exclude", "", "a pattern to prevent attachment of incompatible tags")
	return cmd
}

func newCmdObjectCollectorTagDetach(kind string) *cobra.Command {
	var options commands.CmdObjectCollectorTagDetach
	cmd := &cobra.Command{
		Use:     "detach",
		Short:   "detach a tag from this node",
		Aliases: []string{"deta", "det", "de", "d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	flags.StringVar(&options.Name, "name", "", "the tag name")
	return cmd
}

func newCmdObjectCollectorTagList(kind string) *cobra.Command {
	var options commands.CmdObjectCollectorTagList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list available tags",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdObjectCollectorTagShow(kind string) *cobra.Command {
	var options commands.CmdObjectCollectorTagShow
	cmd := &cobra.Command{
		Use:   "show",
		Short: "show tags attached to this node",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	flags.BoolVar(&options.Verbose, "verbose", false, "also show the attach data")
	return cmd
}

func newCmdObjectCompliance(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "compliance",
		Short:   "node configuration expectations analysis and application",
		Aliases: []string{"compli", "comp", "com", "co"},
	}
}

func newCmdObjectComplianceAttach(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "attach",
		Short:   "attach modulesets and rulesets to the node",
		Aliases: []string{"attac", "atta", "att", "at"},
	}
}

func newCmdObjectComplianceDetach(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "detach",
		Short:   "detach modulesets and rulesets from the node",
		Aliases: []string{"detac", "deta", "det", "de"},
	}
}

func newCmdObjectComplianceList(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "list modules, modulesets and rulesets available",
		Aliases: []string{"ls"},
	}
}

func newCmdObjectInstance(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "instance",
		Short:   "config, status, monitor, list",
		Aliases: []string{"inst", "in"},
	}
}

func newCmdObjectInstanceDevice(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "device",
		Short:   "block device commands",
		Aliases: []string{"dev"},
	}
}

func newCmdObjectComplianceShow(kind string) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "show current modulesets and rulesets attachments, modules last check",
	}
}

func newCmdObjectConfig(kind string) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "object configuration commands",
	}
}

func newCmdObjectEdit(kind string) *cobra.Command {
	var optionsGlobal commands.OptsGlobal
	var optionsConfig commands.CmdObjectConfigEdit
	var optionsKey commands.CmdObjectEditKey
	cmd := &cobra.Command{
		Use:     "edit",
		Short:   "edit object configuration or keystore key",
		Hidden:  true,
		Aliases: []string{"ed"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if optionsKey.Key != "" {
				optionsKey.OptsGlobal = optionsGlobal
				return optionsKey.Run(selectorFlag, kind)
			} else {
				optionsConfig.OptsGlobal = optionsGlobal
				return optionsConfig.Run(selectorFlag, kind)
			}
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &optionsGlobal)
	commoncmd.FlagDiscard(flags, &optionsConfig.Discard)
	commoncmd.FlagRecover(flags, &optionsConfig.Recover)
	commoncmd.FlagKey(flags, &optionsKey.Key)
	cmd.MarkFlagsMutuallyExclusive("discard", "recover")
	cmd.MarkFlagsMutuallyExclusive("discard", "key")
	cmd.MarkFlagsMutuallyExclusive("recover", "key")
	return cmd
}

func newCmdObjectConfigEdit(kind string) *cobra.Command {
	var options commands.CmdObjectConfigEdit
	cmd := &cobra.Command{
		Use:     "edit",
		Short:   "edit selected object and instance configuration",
		Aliases: []string{"ed"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagDiscard(flags, &options.Discard)
	commoncmd.FlagRecover(flags, &options.Recover)
	cmd.MarkFlagsMutuallyExclusive("discard", "recover")
	return cmd
}

func newCmdObjectEditConfig(kind string) *cobra.Command {
	var options commands.CmdObjectConfigEdit
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "edit selected object and instance configuration",
		Hidden:  true,
		Aliases: []string{"conf", "co", "cf", "cfg"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagDiscard(flags, &options.Discard)
	commoncmd.FlagRecover(flags, &options.Recover)
	cmd.MarkFlagsMutuallyExclusive("discard", "recover")
	return cmd
}

func newCmdObjectComplianceAttachModuleset(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceAttachModuleset
	cmd := &cobra.Command{
		Use:     "moduleset",
		Short:   "attach modulesets to this object",
		Long:    "Modules of attached modulesets are checked on schedule.",
		Aliases: []string{"modulese", "modules", "module", "modul", "modu", "mod", "mo"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	return cmd
}

func newCmdObjectComplianceAttachRuleset(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceAttachRuleset
	cmd := &cobra.Command{
		Use:     "ruleset",
		Short:   "attach rulesets to this object",
		Long:    "Rules of attached rulesets are exposed to modules.",
		Aliases: []string{"rulese", "rules", "rule", "rul", "ru"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagRuleset(flags, &options.Ruleset)
	return cmd
}

func newCmdObjectComplianceAuto(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceAuto
	cmd := &cobra.Command{
		Use:   "auto",
		Short: "run modules fixes or checks",
		Long:  "If the module is has the 'autofix' property set, do a fix, else do a check.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModule(flags, &options.Module)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagComplianceAttach(flags, &options.Attach)
	commoncmd.FlagComplianceForce(flags, &options.Force)
	return cmd
}

func newCmdObjectComplianceCheck(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceCheck
	cmd := &cobra.Command{
		Use:     "check",
		Short:   "run modules checks",
		Aliases: []string{"chec", "che", "ch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModule(flags, &options.Module)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagComplianceAttach(flags, &options.Attach)
	commoncmd.FlagComplianceForce(flags, &options.Force)
	return cmd
}

func newCmdObjectComplianceFix(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceFix
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "run modules fixes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModule(flags, &options.Module)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagComplianceAttach(flags, &options.Attach)
	commoncmd.FlagComplianceForce(flags, &options.Force)
	return cmd
}

func newCmdObjectComplianceFixable(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceFixable
	cmd := &cobra.Command{
		Use:     "fixable",
		Short:   "run modules fixable-tests",
		Aliases: []string{"fixabl", "fixab", "fixa"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModule(flags, &options.Module)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagComplianceAttach(flags, &options.Attach)
	commoncmd.FlagComplianceForce(flags, &options.Force)
	return cmd
}

func newCmdObjectComplianceDetachModuleset(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceDetachModuleset
	cmd := &cobra.Command{
		Use:     "moduleset",
		Short:   "detach modulesets from this object",
		Long:    "Modules of attached modulesets are checked on schedule.",
		Aliases: []string{"modulese", "modules", "module", "modul", "modu", "mod", "mo"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	return cmd
}

func newCmdObjectComplianceDetachRuleset(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceDetachRuleset
	cmd := &cobra.Command{
		Use:     "ruleset",
		Short:   "detach rulesets from this object",
		Long:    "Rules of attached rulesets are made available to their module.",
		Aliases: []string{"rulese", "rules", "rule", "rul", "ru"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagRuleset(flags, &options.Ruleset)
	return cmd
}

func newCmdObjectComplianceEnv(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceEnv
	cmd := &cobra.Command{
		Use:     "env",
		Short:   "show the env variables set for a module run",
		Aliases: []string{"en"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	commoncmd.FlagModule(flags, &options.Module)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectComplianceListModules(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceListModules
	cmd := &cobra.Command{
		Use:     "modules",
		Short:   "list modules available on this object",
		Aliases: []string{"module", "modul", "modu", "mod"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdObjectComplianceListModuleset(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceListModuleset
	cmd := &cobra.Command{
		Use:     "moduleset",
		Short:   "list modulesets available to this object",
		Aliases: []string{"modulesets"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	return cmd
}

func newCmdObjectComplianceListRuleset(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceListRuleset
	cmd := &cobra.Command{
		Use:     "ruleset",
		Short:   "list rulesets available to this object",
		Aliases: []string{"rulese", "rules", "rule", "rul", "ru"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagRuleset(flags, &options.Ruleset)
	return cmd
}

func newCmdObjectComplianceShowModuleset(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceShowModuleset
	cmd := &cobra.Command{
		Use:     "moduleset",
		Short:   "show modulesets and modules attached to this object",
		Aliases: []string{"modulese", "modules", "module", "modul", "modu", "mod", "mo"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagModuleset(flags, &options.Moduleset)
	return cmd
}

func newCmdObjectComplianceShowRuleset(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceShowRuleset
	cmd := &cobra.Command{
		Use:     "ruleset",
		Short:   "show rules contextualized for to this object",
		Aliases: []string{"rulese", "rules", "rule", "rul", "ru"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectCreate(kind string) *cobra.Command {
	var options commands.CmdObjectCreate
	cmd := &cobra.Command{
		Use:   "create",
		Short: "create a new object",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagCreateConfig(flags, &options.Config)
	commoncmd.FlagCreateEnv(flags, &options.Env)
	commoncmd.FlagCreateForce(flags, &options.Force)
	commoncmd.FlagCreateNamespace(flags, &options.Namespace)
	commoncmd.FlagCreateRestore(flags, &options.Restore)
	commoncmd.FlagKeywords(flags, &options.Keywords)
	commoncmd.FlagProvision(flags, &options.Provision)
	return cmd
}

func newCmdObjectDelete(kind string) *cobra.Command {
	var options commands.CmdObjectDelete
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del"},
		Short:   "delete configuration object or instances (with --local)",
		Long: "Delete configuration object or instances (with --local)\n\n" +
			"Beware: --local only removes the local instance config." +
			" The config may be recreated by the daemon from a remote instance copy." +
			" Without --local the delete is orchestrated so all instance configurations" +
			" are deleted. The delete command is not responsible for stopping or unprovisioning." +
			" The deletion happens whatever the object status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectDeploy(kind string) *cobra.Command {
	var options commands.CmdObjectCreate
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "create and provision a new object",
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Provision = true
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagCreateConfig(flags, &options.Config)
	commoncmd.FlagCreateEnv(flags, &options.Env)
	commoncmd.FlagCreateForce(flags, &options.Force)
	commoncmd.FlagCreateNamespace(flags, &options.Namespace)
	commoncmd.FlagCreateRestore(flags, &options.Restore)
	commoncmd.FlagKeywords(flags, &options.Keywords)
	return cmd
}

func newCmdObjectConfigDoc(kind string) *cobra.Command {
	var options commands.CmdObjectConfigDoc
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "print the keyword documentation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobalColor(flags, &options.OptsGlobal)
	addFlagsGlobalOutput(flags, &options.OptsGlobal)
	commoncmd.FlagKeyword(flags, &options.Keyword)
	commoncmd.FlagDriver(flags, &options.Driver)
	commoncmd.FlagDepth(flags, &options.Depth)
	return cmd
}

func newCmdObjectDisable(kind string) *cobra.Command {
	var options commands.CmdObjectDisable
	cmd := &cobra.Command{
		Use:   "disable",
		Short: "disable a svc or resources",
		Long:  "Disabled svc or resources are skipped on actions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	return cmd
}

func newCmdObjectEnable(kind string) *cobra.Command {
	var options commands.CmdObjectEnable
	cmd := &cobra.Command{
		Use:   "enable",
		Short: "enable a svc or resources",
		Long:  "Disabled svc or resources are skipped on actions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	return cmd
}

func newCmdObjectEnter(kind string) *cobra.Command {
	var options commands.CmdObjectEnter
	cmd := &cobra.Command{
		Use:   "enter",
		Short: "open a shell in a container resource",
		Long:  "Enter any container resource if --rid is not set.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagObject(flags, &options.ObjectSelector)
	commoncmd.FlagRID(flags, &options.RID)
	return cmd
}

func newCmdObjectConfigEval(kind string) *cobra.Command {
	var options commands.CmdObjectConfigEval
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "evaluate a configuration key value",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagKeywords(flags, &options.Keywords)
	commoncmd.FlagImpersonate(flags, &options.Impersonate)
	cmd.MarkFlagRequired("kw")
	return cmd
}

func newCmdObjectFreeze(kind string) *cobra.Command {
	var options commands.CmdObjectFreeze
	cmd := &cobra.Command{
		Use:   "freeze",
		Short: "block ha automatic start",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectConfigGet(kind string) *cobra.Command {
	var options commands.CmdObjectConfigGet
	cmd := &cobra.Command{
		Use:   "get",
		Short: "get a configuration key value",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagEval(flags, &options.Eval)
	commoncmd.FlagImpersonate(flags, &options.Impersonate)
	commoncmd.FlagKeywords(flags, &options.Keywords)
	return cmd
}

func newCmdObjectGiveback(kind string) *cobra.Command {
	var options commands.CmdObjectGiveback
	cmd := &cobra.Command{
		Use:   "giveback",
		Short: "orchestrate to reach optimal placement",
		Long:  "Stop the misplaced service instances and start on the preferred nodes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	return cmd
}

func newCmdObjectLogs(kind string) *cobra.Command {
	var options commands.CmdObjectLogs
	cmd := &cobra.Command{
		Use:     "logs",
		Aliases: []string{"logs", "log", "lo"},
		Short:   "show object logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLogs(flags, &options.OptsLogs)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectList(kind string) *cobra.Command {
	var options commands.CmdObjectList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "print the selected objects path",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdObjectMonitor(kind string) *cobra.Command {
	var options commands.CmdObjectMonitor
	cmd := &cobra.Command{
		Use:     "monitor",
		Aliases: []string{"mon", "moni", "monit", "monito"},
		Short:   "print the selected objects and instances status summary",
		Long:    monitor.CmdLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagWatch(flags, &options.Watch)
	commoncmd.FlagOutputSections(flags, &options.Sections)
	return cmd
}

func newCmdObjectConfigShow(kind string) *cobra.Command {
	var options commands.CmdObjectConfigShow
	cmd := &cobra.Command{
		Use:   "show",
		Short: "show the object configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagEval(flags, &options.Eval)
	commoncmd.FlagImpersonate(flags, &options.Impersonate)
	return cmd
}

func newCmdObjectConfigMtime(kind string) *cobra.Command {
	var options commands.CmdObjectConfigMtime
	cmd := &cobra.Command{
		Use:     "mtime",
		Short:   "print the object configuration file modification time",
		Aliases: []string{"mtim", "mti", "mt", "m"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdObjectSchedule(kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "schedule",
		Short:   "object scheduler commands",
		Aliases: []string{"sched"},
	}
	return cmd
}

func newCmdObjectScheduleList(kind string) *cobra.Command {
	var options commands.CmdObjectScheduleList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the object scheduler entries",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectInstanceDeviceList(kind string) *cobra.Command {
	var options commands.CmdObjectInstanceDeviceList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "print the object's exposed, used, base and claimed block devices",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagDevRoles(flags, &options.Roles)
	return cmd
}

func newCmdObjectInstanceList(kind string) *cobra.Command {
	var options commands.CmdObjectInstanceList
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "object instances list",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectInstanceStatus(kind string) *cobra.Command {
	var options commands.CmdObjectInstanceStatus
	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"statu", "stat", "sta", "st"},
		Short:   "print the object instances status",
		Long: `Resources Flags:

(1) R   Running,           . Not Running
(2) M   Monitored,         . Not Monitored
(3) D   Disabled,          . Enabled
(4) O   Optional,          . Not Optional
(5) E   Encap,             . Not Encap
(6) P   Not Provisioned,   . Provisioned
(7) S   Standby,           . Not Standby
(8) <n> Remaining Restart, + if more than 10,   . No Restart

`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagRefresh(flags, &options.Refresh)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectProvision(kind string) *cobra.Command {
	var options commands.CmdObjectProvision
	cmd := &cobra.Command{
		Use:     "provision",
		Short:   "allocate system resources for object resources",
		Long:    "For example, provision a fs.ext3 resource means format the device with the mkfs.ext3 command.",
		Aliases: []string{"prov"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagsTo(flags, &options.OptTo)
	commoncmd.FlagForce(flags, &options.Force)
	commoncmd.FlagLeader(flags, &options.Leader)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagDisableRollback(flags, &options.DisableRollback)
	return cmd
}

func newCmdObjectSyncIngest(kind string) *cobra.Command {
	var options commands.CmdObjectSyncIngest
	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "ingest files received from the active instance",
		Long:  "Resource drivers can send files from the active instance to the stand-by instances via the update action.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	return cmd
}

func newCmdObjectPRStart(kind string) *cobra.Command {
	var options commands.CmdObjectPRStart
	cmd := &cobra.Command{
		Use:   "prstart",
		Short: "preempt devices exclusive write access reservation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagsTo(flags, &options.OptTo)
	commoncmd.FlagForce(flags, &options.Force)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectPRStop(kind string) *cobra.Command {
	var options commands.CmdObjectPRStop
	cmd := &cobra.Command{
		Use:   "prstop",
		Short: "release devices exclusive write access reservation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagsTo(flags, &options.OptTo)
	commoncmd.FlagForce(flags, &options.Force)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectPurge(kind string) *cobra.Command {
	var options commands.CmdObjectPurge
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "unprovision and delete",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagsTo(flags, &options.OptTo)
	commoncmd.FlagForce(flags, &options.Force)
	commoncmd.FlagLeader(flags, &options.Leader)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectPushResourceInfo(kind string) *cobra.Command {
	var options commands.CmdObjectResourceInfoPush
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "resinfo",
		Short:  "push key-values reported by resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectRestart(kind string) *cobra.Command {
	var options commands.CmdObjectRestart
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "restart the selected objects, instances or resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagsTo(flags, &options.OptTo)
	commoncmd.FlagForce(flags, &options.Force)
	commoncmd.FlagDisableRollback(flags, &options.DisableRollback)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectSyncFull(kind string) *cobra.Command {
	var options commands.CmdObjectSyncFull
	cmd := &cobra.Command{
		Use:   "full",
		Short: "full copy of the local dataset on peers",
		Long:  "This update can use only full copy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagForce(flags, &options.Force)
	commoncmd.FlagTarget(flags, &options.Target)
	return cmd
}

func newCmdObjectSyncResync(kind string) *cobra.Command {
	var options commands.CmdObjectSyncResync
	cmd := &cobra.Command{
		Use:   "resync",
		Short: "restore optimal synchronization",
		Long:  "Only a subset of drivers support this interface. For example, the disk.md driver re-adds removed disks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagForce(flags, &options.Force)
	return cmd
}

func newCmdObjectSyncUpdate(kind string) *cobra.Command {
	var options commands.CmdObjectSyncUpdate
	cmd := &cobra.Command{
		Use:   "update",
		Short: "synchronize the copy of the local dataset on peers",
		Long:  "This update can use either full or incremental copy, depending on the resource drivers and host capabilities. This is the action executed by the scheduler.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagForce(flags, &options.Force)
	commoncmd.FlagTarget(flags, &options.Target)
	return cmd
}

func newCmdObjectResource(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "resource",
		Short:   "config, status, monitor, list",
		Aliases: []string{"res"},
	}
}

func newCmdObjectResourceInfo(kind string) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "list, push the key-values reported by resources",
	}
}

func newCmdObjectResourceInfoList(kind string) *cobra.Command {
	var options commands.CmdObjectResourceInfoList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the key-values reported by the resources",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectResourceInfoPush(kind string) *cobra.Command {
	var options commands.CmdObjectResourceInfoPush
	cmd := &cobra.Command{
		Use:   "push",
		Short: "push key-values reported by resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectResourceList(kind string) *cobra.Command {
	var options commands.CmdObjectResourceList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the selected resource (config, monitor, status)",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagRID(flags, &options.RID)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectRun(kind string) *cobra.Command {
	var options commands.CmdObjectRun
	cmd := &cobra.Command{
		Use:   "run",
		Short: "run tasks now",
		Long:  "The svc and vol objects can define task resources. Tasks are usually run on a schedule, but this command can trigger a run now.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagConfirm(flags, &options.Confirm)
	commoncmd.FlagCron(flags, &options.Cron)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectSetProvisioned(kind string) *cobra.Command {
	var options commands.CmdObjectSetProvisioned
	cmd := &cobra.Command{
		Use:     "provisioned",
		Short:   "set the resources provisioned property",
		Long:    "This action does not provision the resources (fs are not formatted, disk not allocated, ...). This is just a resources provisioned flag create. Necessary to allow the unprovision action, which is bypassed if the provisioned flag is not set.",
		Aliases: []string{"provision", "prov"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectSetUnprovisioned(kind string) *cobra.Command {
	var options commands.CmdObjectSetUnprovisioned
	cmd := &cobra.Command{
		Use:     "unprovisioned",
		Short:   "unset the resources provisioned property",
		Long:    "This action does not unprovision the resources (fs are not wiped, disk not removed, ...). This is just a resources provisioned flag remove. Necessary to allow the provision action, which is bypassed if the provisioned flag is set.",
		Aliases: []string{"unprovision", "unprov"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectShutdown(kind string) *cobra.Command {
	var options commands.CmdObjectShutdown
	cmd := &cobra.Command{
		Use:   "shutdown",
		Short: "shutdown the object or instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagsTo(flags, &options.OptTo)
	commoncmd.FlagForce(flags, &options.Force)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectStart(kind string) *cobra.Command {
	var options commands.CmdObjectStart
	cmd := &cobra.Command{
		Use:   "start",
		Short: "start objects or instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagsTo(flags, &options.OptTo)
	commoncmd.FlagForce(flags, &options.Force)
	commoncmd.FlagDisableRollback(flags, &options.DisableRollback)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectStartStandby(kind string) *cobra.Command {
	var options commands.CmdObjectStartStandby
	cmd := &cobra.Command{
		Use:   "startstandby",
		Short: "activate resources for standby",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagsTo(flags, &options.OptTo)
	commoncmd.FlagForce(flags, &options.Force)
	commoncmd.FlagDisableRollback(flags, &options.DisableRollback)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectStatus(kind string) *cobra.Command {
	var options commands.CmdObjectStatus
	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"statu", "stat", "sta", "st"},
		Short:   "set the exitcode to the instance status",
		Long:    "This command is silent. Only the exitcode holds information.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagRefresh(flags, &options.Refresh)
	addFlagMonitor(flags, &options.Monitor)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	cmd.MarkFlagsMutuallyExclusive("refresh", "monitor")
	return cmd
}

func newCmdObjectStop(kind string) *cobra.Command {
	var options commands.CmdObjectStop
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "stop objects or instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagsTo(flags, &options.OptTo)
	commoncmd.FlagForce(flags, &options.Force)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectSwitch(kind string) *cobra.Command {
	var options commands.CmdObjectSwitch
	cmd := &cobra.Command{
		Use:   "switch",
		Short: "orchestrate a running instance move-out",
		Long:  "Stop the running object instance and start on the next preferred node.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagSwitchTo(flags, &options.To)
	return cmd
}

func newCmdObjectUnfreeze(kind string) *cobra.Command {
	var options commands.CmdObjectUnfreeze
	cmd := &cobra.Command{
		Use:    "unfreeze",
		Hidden: false,
		Short:  "unblock ha automatic start",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectTakeover(kind string) *cobra.Command {
	var options commands.CmdObjectTakeover
	cmd := &cobra.Command{
		Use:   "takeover",
		Short: "orchestrate a running instance bring-in",
		Long:  "Stop a object instance and start one on the local node.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	return cmd
}

// newCmdObjectThaw creates a hidden 'thaw' subcommand alias for 'unfreeze' (newCmdObjectUnfreeze)
// to unblock ha automatic start.
func newCmdObjectThaw(kind string) *cobra.Command {
	cmd := newCmdObjectUnfreeze(kind)
	cmd.Use = "thaw"
	cmd.Hidden = true
	return cmd
}

func newCmdObjectUnprovision(kind string) *cobra.Command {
	var options commands.CmdObjectUnprovision
	cmd := &cobra.Command{
		Use:     "unprovision",
		Short:   "free system resources (data-loss danger)",
		Aliases: []string{"unprov"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsAsync(flags, &options.OptsAsync)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagsResourceSelector(flags, &options.OptsResourceSelector)
	commoncmd.FlagsTo(flags, &options.OptTo)
	commoncmd.FlagForce(flags, &options.Force)
	commoncmd.FlagLeader(flags, &options.Leader)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectConfigUpdate(kind string) *cobra.Command {
	var options commands.CmdObjectConfigUpdate
	cmd := &cobra.Command{
		Use:   "update",
		Short: "update configuration",
		Long:  "Apply section deletes, keyword unsets then sets. Validate the new configuration and commit.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagUpdateDelete(flags, &options.Delete)
	commoncmd.FlagUpdateSet(flags, &options.Set)
	commoncmd.FlagUpdateUnset(flags, &options.Unset)
	return cmd
}

func newCmdObjectConfigValidate(kind string) *cobra.Command {
	var options commands.CmdObjectConfigValidate
	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "verify the object configuration syntax",
		Aliases: []string{"val", "valid"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	return cmd
}

func newCmdObjectValidateConfig(kind string) *cobra.Command {
	var options commands.CmdObjectConfigValidate
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "verify the object configuration syntax",
		Hidden:  true,
		Aliases: []string{"conf", "co", "cf", "cfg"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	return cmd
}

func newCmdPoolList() *cobra.Command {
	var options commands.CmdPoolList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the cluster pools",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagPoolName(flags, &options.Name)
	return cmd
}

func newCmdPoolVolumeList() *cobra.Command {
	var options commands.CmdPoolVolumeList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the pool volumes",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagPoolName(flags, &options.Name)
	return cmd
}

func newCmdSecGenCert(kind string) *cobra.Command {
	var options commands.CmdSecGenCert
	cmd := &cobra.Command{
		Use:   "gencert",
		Short: "create or replace a x509 certificate stored as a keyset",
		Long:  "Never change an existing private key. Only create a new certificate and renew the certificate chain.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdSecPKCS(kind string) *cobra.Command {
	var options commands.CmdPKCS
	cmd := &cobra.Command{
		Use:   "pkcs",
		Short: "dump the x509 private key and certificate chain in PKCS#12 format",
		Long:  "A sec can contain a certificate, created by the gencert command. The private_key, certificate and certificate_chain are stored as sec keys. The pkcs command decodes the private_key and certificate_chain keys, prepares and print the encrypted, password-protected PKCS#12 format. As this result is bytes-formatted, the stream should be redirected to a file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

// Hidden commands. Kept for backward compatibility.
func newCmdNodeEval() *cobra.Command {
	cmd := newCmdNodeConfigEval()
	cmd.Hidden = true
	return cmd
}

func newCmdNodeGet() *cobra.Command {
	cmd := newCmdNodeConfigGet()
	cmd.Hidden = true
	return cmd
}

func newCmdNodePrintConfig() *cobra.Command {
	cmd := newCmdNodeConfigShow()
	cmd.Use = "config"
	cmd.Hidden = true
	cmd.Aliases = []string{"conf", "co", "cf", "cfg"}
	return cmd
}

func newCmdNodeSet() *cobra.Command {
	var options commands.CmdNodeSet
	cmd := &cobra.Command{
		Use:    "set",
		Short:  "set a configuration key value",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagKeywordOps(flags, &options.KeywordOps)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeUnset() *cobra.Command {
	var options commands.CmdNodeUnset
	cmd := &cobra.Command{
		Use:    "unset",
		Short:  "unset configuration keywords or sections",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagKeywords(flags, &options.Keywords)
	commoncmd.FlagNodeSelector(flags, &options.NodeSelector)
	commoncmd.FlagSections(flags, &options.Sections)
	return cmd
}

func newCmdNodeValidate() *cobra.Command {
	cmd := newCmdNodeConfigValidate()
	cmd.Hidden = true
	cmd.Aliases = []string{"validat", "valida", "valid", "val"}
	return cmd
}

func newCmdNodeVersion() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "version",
		Short:  "display agent version",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			commands.CmdNodeVersion()
		},
	}
	return cmd
}

func newCmdObjectEval(kind string) *cobra.Command {
	cmd := newCmdObjectConfigEval(kind)
	cmd.Hidden = true
	return cmd
}

func newCmdObjectGet(kind string) *cobra.Command {
	cmd := newCmdObjectConfigGet(kind)
	cmd.Hidden = true
	return cmd
}

func newCmdObjectSet(kind string) *cobra.Command {
	var options commands.CmdObjectSet
	cmd := &cobra.Command{
		Use:    "set",
		Short:  "set a configuration key value",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagKeywordOps(flags, &options.KeywordOps)
	return cmd
}

func newCmdObjectUnset(kind string) *cobra.Command {
	var options commands.CmdObjectUnset
	cmd := &cobra.Command{
		Use:    "unset",
		Short:  "unset configuration keywords or sections",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	commoncmd.FlagsLock(flags, &options.OptsLock)
	commoncmd.FlagKeywords(flags, &options.Keywords)
	commoncmd.FlagSections(flags, &options.Sections)
	return cmd
}

func newCmdObjectValidate(kind string) *cobra.Command {
	cmd := newCmdObjectConfigValidate(kind)
	cmd.Hidden = true
	cmd.Aliases = []string{"validat", "valida", "valid", "vali", "val"}
	return cmd
}

func newCmdObjectPrintStatus(kind string) *cobra.Command {
	cmd := newCmdObjectInstanceStatus(kind)
	cmd.Hidden = true
	return cmd
}

func newCmdObjectPrintSchedule(kind string) *cobra.Command {
	cmd := newCmdObjectScheduleList(kind)
	cmd.Hidden = true
	cmd.Use = "schedule"
	cmd.Aliases = []string{"schedul", "schedu", "sched", "sche", "sch", "sc"}
	return cmd
}

func newCmdObjectPrintConfig(kind string) *cobra.Command {
	cmd := newCmdObjectConfigShow(kind)
	cmd.Use = "config"
	cmd.Hidden = true
	cmd.Aliases = []string{"conf", "co", "cf", "cfg"}
	return cmd
}
