package ox

import (
	// Necessary to use go:embed
	_ "embed"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/monitor"
	commands "github.com/opensvc/om3/core/oxcmd"
	"github.com/opensvc/om3/core/tui"
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

func newCmdArrayLs() *cobra.Command {
	var options commands.CmdArrayLs
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "list the cluster-managed storage arrays",
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
	addFlagsAsync(flags, &options.OptsAsync)
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
	addFlagsAsync(flags, &options.OptsAsync)
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
	addFlagsLogs(flags, &options.OptsLogs)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdClusterThaw() *cobra.Command {
	var options commands.CmdClusterUnfreeze
	cmd := &cobra.Command{
		Use:    "thaw",
		Hidden: true,
		Short:  "unblock ha automatic and split action start on all nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsAsync(flags, &options.OptsAsync)
	return cmd
}

func newCmdClusterUnfreeze() *cobra.Command {
	var options commands.CmdClusterUnfreeze
	cmd := &cobra.Command{
		Use:   "unfreeze",
		Short: "unblock ha automatic and split action start on all nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsAsync(flags, &options.OptsAsync)
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
	addFlagRoles(flags, &options.Roles)
	flags.DurationVar(&options.Duration, "duration", 60*time.Second, "token duration.")
	flags.StringSliceVar(&options.Out, "out", []string{"token"}, "the fields to display: [token,expired_at]")
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
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdDaemonShutdown() *cobra.Command {
	var options commands.CmdDaemonShutdown
	cmd := &cobra.Command{
		Use:   "shutdown",
		Short: "Shutdown all local svc and vol objects then shutdown the daemon.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagDuration(flags, &options.Timeout)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdDaemonStatus() *cobra.Command {
	var options commands.CmdObjectMonitor
	//var options commands.CmdDaemonStatus
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "print the cluster status",
		Long:    monitor.CmdLong,
		Aliases: []string{"statu"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run("**", "")
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagWatch(flags, &options.Watch)
	addFlagOutputSections(flags, &options.Sections)
	return cmd
}

func newCmdDaemonStats() *cobra.Command {
	var options commands.CmdDaemonStats
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "print the resource usage statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagKey(flags, &options.Key)
	addFlagFrom(flags, &from)
	addFlagValue(flags, &value)
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
	addFlagKey(flags, &options.Key)
	addFlagFrom(flags, &from)
	addFlagValue(flags, &value)
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
	addFlagKey(flags, &options.Key)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagKey(flags, &options.Key)
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
	addFlagMatch(flags, &options.Match)
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
	addFlagKey(flags, &options.Key)
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
	addFlagKey(flags, &options.Key)
	addFlagKeyTo(flags, &options.To)
	return cmd
}

func newCmdMonitor() *cobra.Command {
	var options commands.CmdObjectMonitor
	cmd := &cobra.Command{
		Use:     "monitor",
		Aliases: []string{"m", "mo", "mon", "moni", "monit", "monito"},
		Short:   "Print the cluster status",
		Long:    monitor.CmdLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run("*", "")
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagWatch(flags, &options.Watch)
	addFlagOutputSections(flags, &options.Sections)
	return cmd
}

func newCmdNetworkLs() *cobra.Command {
	var options commands.CmdNetworkLs
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "list the cluster networks",
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

func newCmdNetworkIPLs() *cobra.Command {
	var options commands.CmdNetworkIPLs
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "list the ip in the cluster networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNetworkStatusName(flags, &options.Name)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeCapabilitiesList() *cobra.Command {
	var options commands.CmdNodeCapabilitiesList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the node capabilities",
		Aliases: []string{"lis", "li", "ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
	flags.StringVar(&options.Name, "name", "", "the tag name")
	return cmd
}

func newCmdNodeCollectorTagList() *cobra.Command {
	var options commands.CmdNodeCollectorTagList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list available tags",
		Aliases: []string{"lis", "li", "ls", "l"},
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
		Use:     "show",
		Short:   "show tags attached to this node",
		Aliases: []string{"sho", "sh", "s"},
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
	addFlagModuleset(flags, &options.Moduleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagRuleset(flags, &options.Ruleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagModule(flags, &options.Module)
	addFlagModuleset(flags, &options.Moduleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagComplianceAttach(flags, &options.Attach)
	addFlagComplianceForce(flags, &options.Force)
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
	addFlagModule(flags, &options.Module)
	addFlagModuleset(flags, &options.Moduleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagComplianceAttach(flags, &options.Attach)
	addFlagComplianceForce(flags, &options.Force)
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
	addFlagModule(flags, &options.Module)
	addFlagModuleset(flags, &options.Moduleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagComplianceAttach(flags, &options.Attach)
	addFlagComplianceForce(flags, &options.Force)
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
	addFlagModule(flags, &options.Module)
	addFlagModuleset(flags, &options.Moduleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagComplianceAttach(flags, &options.Attach)
	addFlagComplianceForce(flags, &options.Force)
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
	addFlagModuleset(flags, &options.Moduleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagRuleset(flags, &options.Ruleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagModuleset(flags, &options.Moduleset)
	addFlagModule(flags, &options.Module)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagModuleset(flags, &options.Moduleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagRuleset(flags, &options.Ruleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagModuleset(flags, &options.Moduleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodePing() *cobra.Command {
	var options commands.CmdNodePing
	cmd := &cobra.Command{
		Use:   "ping",
		Short: "ask node to ping all cluster nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeSystem() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "node system commands",
	}
	return cmd
}

func newCmdNodeSystemDisk() *cobra.Command {
	var options commands.CmdNodeSystemDisk
	cmd := &cobra.Command{
		Use:     "disk",
		Short:   "show node system disks",
		Aliases: []string{"dsk", "dis"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeSystemGroup() *cobra.Command {
	var options commands.CmdNodeSystemGroup
	cmd := &cobra.Command{
		Use:     "group",
		Short:   "show node system groups",
		Aliases: []string{"grp", "gr"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeSystemHardware() *cobra.Command {
	var options commands.CmdNodeSystemHardware
	cmd := &cobra.Command{
		Use:     "hardware",
		Short:   "show node system hardware",
		Aliases: []string{"device", "hard"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeSystemIPAddress() *cobra.Command {
	var options commands.CmdNodeSystemIPAddress
	cmd := &cobra.Command{
		Use:     "ipaddress",
		Short:   "show node system IP address",
		Aliases: []string{"addr", "ipaddr"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeSystemPackage() *cobra.Command {
	var options commands.CmdNodeSystemPackage
	cmd := &cobra.Command{
		Use:     "package",
		Short:   "show node system package",
		Aliases: []string{"pkg"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeSystemPatch() *cobra.Command {
	var options commands.CmdNodeSystemPatch
	cmd := &cobra.Command{
		Use:     "patch",
		Short:   "show node system patch",
		Aliases: []string{"pacth"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeSystemProperty() *cobra.Command {
	var options commands.CmdNodeSystemProperty
	cmd := &cobra.Command{
		Use:     "property",
		Short:   "show node system property",
		Aliases: []string{"proper", "prop"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeSystemSAN() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "san",
		Short: "node system san commands",
	}
	return cmd
}

func newCmdNodeSystemSANPathInitiator() *cobra.Command {
	var options commands.CmdNodeSystemInitiator
	cmd := &cobra.Command{
		Use:     "initiator",
		Short:   "show node system san initiator",
		Aliases: []string{"init", "ini"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeSystemSANPath() *cobra.Command {
	var options commands.CmdNodeSystemSANPath
	cmd := &cobra.Command{
		Use:     "path",
		Short:   "show node system san path",
		Aliases: []string{"pa"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeSystemUser() *cobra.Command {
	var options commands.CmdNodeSystemUser
	cmd := &cobra.Command{
		Use:     "user",
		Short:   "show node system users",
		Aliases: []string{"usr", "us"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeEdit() *cobra.Command {
	var options commands.CmdNodeEditConfig
	cmd := &cobra.Command{
		Use:     "edit",
		Short:   "edit the node configuration",
		Aliases: []string{"ed", "edi"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagDiscard(flags, &options.Discard)
	addFlagRecover(flags, &options.Recover)
	cmd.MarkFlagsMutuallyExclusive("discard", "recover")
	return cmd
}

func newCmdNodeEditConfig() *cobra.Command {
	var options commands.CmdNodeEditConfig
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "edit the node configuration",
		Aliases: []string{"confi", "conf", "con", "co", "c", "cf", "cfg"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagDiscard(flags, &options.Discard)
	addFlagRecover(flags, &options.Recover)
	cmd.MarkFlagsMutuallyExclusive("discard", "recover")
	return cmd
}

func newCmdNodeEval() *cobra.Command {
	var options commands.CmdNodeEval
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "evaluate a configuration key value",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsLock(flags, &options.OptsLock)
	addFlagImpersonate(flags, &options.Impersonate)
	addFlagKeywords(flags, &options.Keywords)
	addFlagNodeSelector(flags, &options.NodeSelector)
	cmd.MarkFlagRequired("kw")
	return cmd
}

func newCmdNodeEvents() *cobra.Command {
	var options commands.CmdNodeEvents
	cmd := &cobra.Command{
		Use:     "events",
		Short:   "print the node event stream",
		Long:    "print the node event stream\n\nAvailable kinds: \n" + eventKindTemplate,
		Aliases: []string{"eve", "even", "event"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagEventFilters(flags, &options.Filters)
	addFlagDuration(flags, &options.Duration)
	addFlagEventTemplate(flags, &options.Template)
	addFlagWait(flags, &options.Wait)
	addFlagNodeSelector(flags, &options.NodeSelector)
	flags.Uint64Var(&options.Limit, "limit", 0, "limit event count to fetch, set to 1 when --wait is used")
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
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeGet() *cobra.Command {
	var options commands.CmdNodeGet
	cmd := &cobra.Command{
		Use:   "get",
		Short: "get a configuration key value",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsLock(flags, &options.OptsLock)
	addFlagEval(flags, &options.Eval)
	addFlagImpersonate(flags, &options.Impersonate)
	addFlagKeywords(flags, &options.Keywords)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsLogs(flags, &options.OptsLogs)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeLs() *cobra.Command {
	var options commands.CmdNodeLs
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "list the cluster nodes",
		Long:  "The list can be filtered using the --node selector. This command can be used to validate node selector expressions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodePrintConfig() *cobra.Command {
	var options commands.CmdNodePrintConfig
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "print the node configuration",
		Aliases: []string{"confi", "conf", "con", "co", "c", "cf", "cfg"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagEval(flags, &options.Eval)
	addFlagImpersonate(flags, &options.Impersonate)
	return cmd
}

func newCmdNodePrintSchedule() *cobra.Command {
	var options commands.CmdNodePrintSchedule
	cmd := &cobra.Command{
		Use:     "schedule",
		Short:   "print selected objects scheduling table",
		Aliases: []string{"schedul", "schedu", "sched", "sche", "sch", "sc"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagCollectorUser(flags, &options.User)
	addFlagCollectorPassword(flags, &options.Password)
	addFlagCollectorApp(flags, &options.App)
	addFlagNodeSelector(flags, &options.NodeSelector)

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
	addFlagRelay(flagSet, &options.Relays)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagKeywordOps(flags, &options.KeywordOps)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeUpdate() *cobra.Command {
	var options commands.CmdNodeUpdate
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagUpdateDelete(flags, &options.Delete)
	addFlagUpdateSet(flags, &options.Set)
	addFlagUpdateUnset(flags, &options.Unset)
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
	addFlagForce(flags, &options.Force)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagKeywords(flags, &options.Keywords)
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagSections(flags, &options.Sections)
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

func newCmdNodeValidate() *cobra.Command {
	var options commands.CmdNodeValidateConfig
	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "verify the node configuration syntax",
		Aliases: []string{"validat", "valida", "valid", "val"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdNodeValidateConfig() *cobra.Command {
	var options commands.CmdNodeValidateConfig
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "verify the node configuration syntax",
		Aliases: []string{"confi", "conf", "con", "co", "c"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdObjectBoot(kind string) *cobra.Command {
	var options commands.CmdObjectBoot
	cmd := &cobra.Command{
		Use: "boot",
		Short: "Clean up actions executed before the daemon starts." +
			" For example scsi reservation release and vg tags removal." +
			" Never execute this action manually.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdNodeSSHTrust() *cobra.Command {
	var options commands.CmdNodeSSHTrust
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "ssh-trust peer nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdClusterSSHTrust() *cobra.Command {
	var options commands.CmdClusterSSHTrust
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "setup the ssh access trust mesh",
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

func newCmdObjectValidate(kind string) *cobra.Command {
	var options commands.CmdObjectValidateConfig
	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "verify the object configuration syntax",
		Aliases: []string{"validat", "valida", "valid", "vali", "val"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsLock(flags, &options.OptsLock)
	return cmd
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

func newCmdObjectCollectorTagShow(kind string) *cobra.Command {
	var options commands.CmdObjectCollectorTagShow
	cmd := &cobra.Command{
		Use:     "show",
		Short:   "show tags attached to this node",
		Aliases: []string{"sho", "sh", "s"},
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
		Aliases: []string{"lis", "li", "ls", "l"},
	}
}

func newCmdObjectInstance(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "instance",
		Short:   "config, status, monitor, ls",
		Aliases: []string{"inst", "in"},
	}
}

func newCmdObjectComplianceShow(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Short:   "show current modulesets and rulesets attachments, modules last check",
		Aliases: []string{"sho", "sh", "s"},
	}
}

func newCmdObjectEdit(kind string) *cobra.Command {
	var optionsGlobal commands.OptsGlobal
	var optionsConfig commands.CmdObjectEditConfig
	var optionsKey commands.CmdObjectEditKey
	cmd := &cobra.Command{
		Use:     "edit",
		Short:   "edit object configuration or keystore key",
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
	addFlagDiscard(flags, &optionsConfig.Discard)
	addFlagRecover(flags, &optionsConfig.Recover)
	addFlagKey(flags, &optionsKey.Key)
	cmd.MarkFlagsMutuallyExclusive("discard", "recover")
	cmd.MarkFlagsMutuallyExclusive("discard", "key")
	cmd.MarkFlagsMutuallyExclusive("recover", "key")
	return cmd
}

func newCmdObjectEditConfig(kind string) *cobra.Command {
	var options commands.CmdObjectEditConfig
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "edit selected object and instance configuration",
		Aliases: []string{"conf", "c", "cf", "cfg"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagDiscard(flags, &options.Discard)
	addFlagRecover(flags, &options.Recover)
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
	addFlagModuleset(flags, &options.Moduleset)
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
	addFlagRuleset(flags, &options.Ruleset)
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
	addFlagModule(flags, &options.Module)
	addFlagModuleset(flags, &options.Moduleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagComplianceAttach(flags, &options.Attach)
	addFlagComplianceForce(flags, &options.Force)
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
	addFlagModule(flags, &options.Module)
	addFlagModuleset(flags, &options.Moduleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagComplianceAttach(flags, &options.Attach)
	addFlagComplianceForce(flags, &options.Force)
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
	addFlagModule(flags, &options.Module)
	addFlagModuleset(flags, &options.Moduleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagComplianceAttach(flags, &options.Attach)
	addFlagComplianceForce(flags, &options.Force)
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
	addFlagModule(flags, &options.Module)
	addFlagModuleset(flags, &options.Moduleset)
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagComplianceAttach(flags, &options.Attach)
	addFlagComplianceForce(flags, &options.Force)
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
	addFlagModuleset(flags, &options.Moduleset)
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
	addFlagRuleset(flags, &options.Ruleset)
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
	addFlagModuleset(flags, &options.Moduleset)
	addFlagModule(flags, &options.Module)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagModuleset(flags, &options.Moduleset)
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
	addFlagRuleset(flags, &options.Ruleset)
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
	addFlagModuleset(flags, &options.Moduleset)
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
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagsLock(flags, &options.OptsLock)
	addFlagCreateConfig(flags, &options.Config)
	addFlagCreateForce(flags, &options.Force)
	addFlagCreateNamespace(flags, &options.Namespace)
	addFlagCreateRestore(flags, &options.Restore)
	addFlagKeywords(flags, &options.Keywords)
	addFlagEnv(flags, &options.Env)
	addFlagInteractive(flags, &options.Interactive)
	addFlagProvision(flags, &options.Provision)
	return cmd
}

func newCmdObjectDelete(kind string) *cobra.Command {
	var options commands.CmdObjectDelete
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del"},
		Short:   "delete configuration object or instances (with --local)",
		Long: "delete configuration object or instances (with --local)\n\n" +
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagsLock(flags, &options.OptsLock)
	addFlagCreateConfig(flags, &options.Config)
	addFlagCreateForce(flags, &options.Force)
	addFlagCreateNamespace(flags, &options.Namespace)
	addFlagCreateRestore(flags, &options.Restore)
	addFlagKeywords(flags, &options.Keywords)
	addFlagEnv(flags, &options.Env)
	addFlagInteractive(flags, &options.Interactive)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
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
	addFlagRID(flags, &options.RID)
	return cmd
}

func newCmdObjectEval(kind string) *cobra.Command {
	var options commands.CmdObjectEval
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "evaluate a configuration key value",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagKeywords(flags, &options.Keywords)
	addFlagImpersonate(flags, &options.Impersonate)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectGet(kind string) *cobra.Command {
	var options commands.CmdObjectGet
	cmd := &cobra.Command{
		Use:   "get",
		Short: "get a configuration key value",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagEval(flags, &options.Eval)
	addFlagImpersonate(flags, &options.Impersonate)
	addFlagKeywords(flags, &options.Keywords)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagsLock(flags, &options.OptsLock)
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
	addFlagsLogs(flags, &options.OptsLogs)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectLs(kind string) *cobra.Command {
	var options commands.CmdObjectLs
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "print the selected objects path",
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
	addFlagWatch(flags, &options.Watch)
	addFlagOutputSections(flags, &options.Sections)
	return cmd
}

func newCmdObjectPrintConfig(kind string) *cobra.Command {
	var options commands.CmdObjectPrintConfig
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "print the object configuration",
		Aliases: []string{"confi", "conf", "con", "co", "c", "cf", "cfg"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagEval(flags, &options.Eval)
	addFlagImpersonate(flags, &options.Impersonate)
	return cmd
}

func newCmdObjectPrintDevices(kind string) *cobra.Command {
	var options commands.CmdObjectPrintDevices
	cmd := &cobra.Command{
		Use:     "devices",
		Short:   "print the object's exposed, used, base and claimed block devices",
		Aliases: []string{"device", "devic", "devi", "dev", "devs", "de"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagDevRoles(flags, &options.Roles)
	return cmd
}

func newCmdObjectPrintResourceInfo(kind string) *cobra.Command {
	var options commands.CmdObjectPrintResourceInfo
	cmd := &cobra.Command{
		Use:   "resinfo",
		Short: "print all objects resource info",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectPrintSchedule(kind string) *cobra.Command {
	var options commands.CmdObjectPrintSchedule
	cmd := &cobra.Command{
		Use:     "schedule",
		Short:   "print the objects scheduling table",
		Aliases: []string{"schedul", "schedu", "sched", "sche", "sch", "sc"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectInstanceLs(kind string) *cobra.Command {
	var options commands.CmdObjectInstanceLs
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "object instances list",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectPrintStatus(kind string) *cobra.Command {
	var options commands.CmdObjectPrintStatus
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagRefresh(flags, &options.Refresh)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagsTo(flags, &options.OptTo)
	addFlagForce(flags, &options.Force)
	addFlagLeader(flags, &options.Leader)
	addFlagNodeSelector(flags, &options.NodeSelector)
	addFlagDisableRollback(flags, &options.DisableRollback)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagsTo(flags, &options.OptTo)
	addFlagForce(flags, &options.Force)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagsTo(flags, &options.OptTo)
	addFlagForce(flags, &options.Force)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagsTo(flags, &options.OptTo)
	addFlagForce(flags, &options.Force)
	addFlagLeader(flags, &options.Leader)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectPushResourceInfo(kind string) *cobra.Command {
	var options commands.CmdObjectPushResourceInfo
	cmd := &cobra.Command{
		Use:     "resinfo",
		Short:   "push resource info key/val pairs",
		Aliases: []string{"res"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagsTo(flags, &options.OptTo)
	addFlagForce(flags, &options.Force)
	addFlagDisableRollback(flags, &options.DisableRollback)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectSyncIngest(kind string) *cobra.Command {
	var options commands.CmdObjectSyncIngest
	cmd := &cobra.Command{
		Use:   "Ingest",
		Short: "ingest files received from the active instance",
		Long:  "Resource drivers can send files from the active instance to the stand-by instances via the update action.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagForce(flags, &options.Force)
	addFlagTarget(flags, &options.Target)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagForce(flags, &options.Force)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagForce(flags, &options.Force)
	addFlagTarget(flags, &options.Target)
	return cmd
}

func newCmdObjectResource(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "resource",
		Short:   "config, status, monitor, ls",
		Aliases: []string{"res"},
	}
}

func newCmdObjectResourceLs(kind string) *cobra.Command {
	var options commands.CmdObjectResourceLs
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "list the selected resource (config, monitor, status)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagRID(flags, &options.RID)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagConfirm(flags, &options.Confirm)
	addFlagCron(flags, &options.Cron)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagKeywordOps(flags, &options.KeywordOps)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagsTo(flags, &options.OptTo)
	addFlagForce(flags, &options.Force)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagsTo(flags, &options.OptTo)
	addFlagForce(flags, &options.Force)
	addFlagDisableRollback(flags, &options.DisableRollback)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagsTo(flags, &options.OptTo)
	addFlagForce(flags, &options.Force)
	addFlagDisableRollback(flags, &options.DisableRollback)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagRefresh(flags, &options.Refresh)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagsTo(flags, &options.OptTo)
	addFlagForce(flags, &options.Force)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagsLock(flags, &options.OptsLock)
	addFlagSwitchTo(flags, &options.To)
	return cmd
}

func newCmdObjectUnfreeze(kind string) *cobra.Command {
	var options commands.CmdObjectUnfreeze
	cmd := &cobra.Command{
		Use:   "unfreeze",
		Short: "unblock ha automatic start",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagNodeSelector(flags, &options.NodeSelector)
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
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagsLock(flags, &options.OptsLock)
	return cmd
}

func newCmdObjectThaw(kind string) *cobra.Command {
	var options commands.CmdObjectUnfreeze
	cmd := &cobra.Command{
		Use:    "thaw",
		Hidden: true,
		Short:  "unblock ha automatic start",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectUnprovision(kind string) *cobra.Command {
	var options commands.CmdObjectUnprovision
	cmd := &cobra.Command{
		Use:     "unprovision",
		Short:   "free system resources (data-loss danger).",
		Aliases: []string{"unprov"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsAsync(flags, &options.OptsAsync)
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagsTo(flags, &options.OptTo)
	addFlagForce(flags, &options.Force)
	addFlagLeader(flags, &options.Leader)
	addFlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func newCmdObjectUpdate(kind string) *cobra.Command {
	var options commands.CmdObjectUpdate
	cmd := &cobra.Command{
		Use:    "update",
		Short:  "update configuration",
		Long:   "Apply section deletes, keyword unsets then sets. Validate the new configuration and commit.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsLock(flags, &options.OptsLock)
	addFlagUpdateDelete(flags, &options.Delete)
	addFlagUpdateSet(flags, &options.Set)
	addFlagUpdateUnset(flags, &options.Unset)
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagKeywords(flags, &options.Keywords)
	addFlagSections(flags, &options.Sections)
	return cmd
}

func newCmdObjectValidateConfig(kind string) *cobra.Command {
	var options commands.CmdObjectValidateConfig
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "verify the object configuration syntax",
		Aliases: []string{"confi", "conf", "con", "co", "c"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsLock(flags, &options.OptsLock)
	return cmd
}

func newCmdPoolLs() *cobra.Command {
	var options commands.CmdPoolLs
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "list the cluster pools",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagPoolName(flags, &options.Name)
	return cmd
}

func newCmdPoolVolumeLs() *cobra.Command {
	var options commands.CmdPoolVolumeLs
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "list the pool volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagPoolName(flags, &options.Name)
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

func newCmdTUI(kind string) *cobra.Command {
	var options tui.Options
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "interactive terminal user interface.",
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Selector = mergeSelector("", kind, "")
			return tui.Run(&options)
		},
	}
	return cmd
}
