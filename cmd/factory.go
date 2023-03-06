package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/commands"
	"github.com/opensvc/om3/core/monitor"
)

func newCmdAll() *cobra.Command {
	return &cobra.Command{
		Hidden: false,
		Use:    "all",
		Short:  "manage a mix of objects, tentatively exposing all commands",
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
	flags.StringArrayVar(&options.Out, "out", []string{}, "output 'token' or 'token_expire_at'")
	return cmd
}

func newCmdDaemonDNSDump() *cobra.Command {
	var options commands.CmdDaemonDNSDump
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
	flags.StringVar(&options.Token, "token", "", "auth token with 'join' role"+
		" (created from 'om daemon auth --role json')")
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
	flags.DurationVar(&options.Timeout, "timeout", 5*time.Second, "maximum duration to wait for local node removed from cluster")
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
		Short:   "restart the daemon or a daemon subsystem",
		Aliases: []string{"restart"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagForeground(flags, &options.Foreground)
	addFlagDebug(flags, &options.Debug)
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
	addFlagDebug(flags, &options.Debug)
	addFlagForeground(flags, &options.Foreground)
	addFlagCpuProfile(flags, &options.CpuProfile)
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
	addFlagSections(flags, &options.Sections)
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
	return cmd
}

func newCmdKeystoreAdd(kind string) *cobra.Command {
	var options commands.CmdKeystoreAdd
	cmd := &cobra.Command{
		Use:   "add",
		Short: "add new keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsLock(flags, &options.OptsLock)
	addFlagKey(flags, &options.Key)
	addFlagFrom(flags, &options.From)
	addFlagValue(flags, &options.Value)
	cmd.MarkFlagsMutuallyExclusive("from", "value")
	return cmd
}

func newCmdKeystoreChange(kind string) *cobra.Command {
	var options commands.CmdKeystoreChange
	cmd := &cobra.Command{
		Use:   "change",
		Short: "change existing keys value",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagKey(flags, &options.Key)
	addFlagFrom(flags, &options.From)
	addFlagValue(flags, &options.Value)
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
	addFlagSections(flags, &options.Sections)
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

func newCmdNetworkStatus() *cobra.Command {
	var options commands.CmdNetworkStatus
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "show the cluster networks usage",
		Aliases: []string{"statu", "stat", "sta", "st"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagNetworkStatusName(flags, &options.Name)
	addFlagNetworkStatusVerbose(flags, &options.Verbose)
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
	addFlagsGlobal(flags, &options.OptsGlobal)
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
	return cmd
}

func newCmdNodeDelete() *cobra.Command {
	var options commands.CmdNodeDelete
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "delete a configuration section",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagRID(flags, &options.RID)
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
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagKeyword(flags, &options.Keyword)
	addFlagDriver(flags, &options.Driver)
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
	addFlagsAsync(flags, &options.OptsAsync)
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
	addFlagKeyword(flags, &options.Keyword)
	return cmd
}

func newCmdNodeEvents() *cobra.Command {
	var options commands.CmdNodeEvents
	cmd := &cobra.Command{
		Use:     "events",
		Short:   "print the node event stream",
		Aliases: []string{"eve", "even", "event"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagEventFilters(flags, &options.Filters)
	addFlagDuration(flags, &options.Duration)
	flags.Uint64Var(&options.Limit, "limit", 0, "limit event count to fetch")
	return cmd
}

func newCmdNodeFreeze() *cobra.Command {
	var options commands.CmdNodeFreeze
	cmd := &cobra.Command{
		Use:   "freeze",
		Short: "block ha automatic start",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsAsync(flags, &options.OptsAsync)
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
	addFlagKeyword(flags, &options.Keyword)
	return cmd
}

func newCmdNodeLogs() *cobra.Command {
	var options commands.CmdNodeLogs
	cmd := &cobra.Command{
		Use:     "logs",
		Aliases: []string{"logs", "log", "lo"},
		Short:   "Filter and format logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagLogsFollow(flags, &options.Follow)
	addFlagLogsSID(flags, &options.SID)
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
	return cmd
}

func newCmdNodePushDisks() *cobra.Command {
	var options commands.CmdNodePushDisks
	cmd := &cobra.Command{
		Use:     "disks",
		Short:   "run the disk discovery, push and print the result",
		Aliases: []string{"disk", "dis", "di"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
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
		Use:   "set",
		Short: "set a configuration key value",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsLock(flags, &options.OptsLock)
	addFlagKeywordOps(flags, &options.KeywordOps)
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
	return cmd
}

func newCmdNodeThaw() *cobra.Command {
	var options commands.CmdNodeUnfreeze
	cmd := &cobra.Command{
		Use:     "thaw",
		Short:   "unblock ha automatic start",
		Hidden:  true,
		Aliases: []string{"thaw"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsAsync(flags, &options.OptsAsync)
	return cmd
}

func newCmdNodeUnfreeze() *cobra.Command {
	var options commands.CmdNodeUnfreeze
	cmd := &cobra.Command{
		Use:     "unfreeze",
		Short:   "unblock ha automatic start",
		Aliases: []string{"thaw"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsAsync(flags, &options.OptsAsync)
	return cmd
}

func newCmdNodeUnset() *cobra.Command {
	var options commands.CmdNodeUnset
	cmd := &cobra.Command{
		Use:   "unset",
		Short: "unset a configuration key",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsLock(flags, &options.OptsLock)
	addFlagKeywords(flags, &options.Keywords)
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
	return &cobra.Command{
		Use:     "validate",
		Short:   "Validation command group",
		Aliases: []string{"validat", "valida", "valid", "vali", "val"},
	}
}

func newCmdObjectSync(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "sync",
		Short:   "data synchronization command group",
		Aliases: []string{"syn", "sy"},
	}
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
		Use:   "delete",
		Short: "delete objects, instances or configuration sections",
		Long:  "Beware: not setting --local nor --rid deletes all object instances via orchestration, which leaves no local backup of the configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsLock(flags, &options.OptsLock)
	addFlagDryRun(flags, &options.DryRun)
	addFlagRID(flags, &options.RID)
	return cmd
}

func newCmdObjectDoc(kind string) *cobra.Command {
	var options commands.CmdObjectDoc
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "print the documentation of the selected keywords",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagKeyword(flags, &options.Keyword)
	addFlagDriver(flags, &options.Driver)
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
	addFlagKeyword(flags, &options.Keyword)
	addFlagImpersonate(flags, &options.Impersonate)
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
	addFlagKeyword(flags, &options.Keyword)
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
		Short:   "filter and format logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagLogsFollow(flags, &options.Follow)
	addFlagLogsSID(flags, &options.SID)
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
	addFlagSections(flags, &options.Sections)
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

func newCmdObjectPrintConfigMtime(kind string) *cobra.Command {
	var options commands.CmdObjectPrintConfigMtime
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
	addFlagDevRoles(flags, &options.Roles)
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
	addFlagDryRun(flags, &options.DryRun)
	addFlagForce(flags, &options.Force)
	addFlagLeader(flags, &options.Leader)
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
	addFlagDryRun(flags, &options.DryRun)
	addFlagForce(flags, &options.Force)
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
	addFlagDryRun(flags, &options.DryRun)
	addFlagForce(flags, &options.Force)
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
	addFlagDryRun(flags, &options.DryRun)
	addFlagForce(flags, &options.Force)
	addFlagLeader(flags, &options.Leader)
	return cmd
}

func newCmdObjectPushResInfo(kind string) *cobra.Command {
	var options commands.CmdObjectPushResInfo
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
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagsTo(flags, &options.OptTo)
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
	addFlagDryRun(flags, &options.DryRun)
	addFlagForce(flags, &options.Force)
	addFlagDisableRollback(flags, &options.DisableRollback)
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
	addFlagDryRun(flags, &options.DryRun)
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
	addFlagDryRun(flags, &options.DryRun)
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
	addFlagDryRun(flags, &options.DryRun)
	addFlagForce(flags, &options.Force)
	addFlagTarget(flags, &options.Target)
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
	addFlagDryRun(flags, &options.DryRun)
	addFlagConfirm(flags, &options.Confirm)
	addFlagCron(flags, &options.Cron)
	return cmd
}

func newCmdObjectSet(kind string) *cobra.Command {
	var options commands.CmdObjectSet
	cmd := &cobra.Command{
		Use:   "set",
		Short: "set a configuration key value",
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
	addFlagDryRun(flags, &options.DryRun)
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
	addFlagDryRun(flags, &options.DryRun)
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
	addFlagDryRun(flags, &options.DryRun)
	addFlagForce(flags, &options.Force)
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
	addFlagDryRun(flags, &options.DryRun)
	addFlagForce(flags, &options.Force)
	addFlagDisableRollback(flags, &options.DisableRollback)
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
	addFlagDryRun(flags, &options.DryRun)
	addFlagForce(flags, &options.Force)
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
	addFlagDryRun(flags, &options.DryRun)
	addFlagForce(flags, &options.Force)
	addFlagLeader(flags, &options.Leader)
	return cmd
}

func newCmdObjectUnset(kind string) *cobra.Command {
	var options commands.CmdObjectUnset
	cmd := &cobra.Command{
		Use:   "unset",
		Short: "unset a configuration key",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsLock(flags, &options.OptsLock)
	addFlagKeywords(flags, &options.Keywords)
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
	return cmd
}

func newCmdPoolStatus() *cobra.Command {
	var options commands.CmdPoolStatus
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "show the cluster pools usage",
		Aliases: []string{"statu", "stat", "sta", "st"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagPoolStatusName(flags, &options.Name)
	addFlagPoolStatusVerbose(flags, &options.Verbose)
	return cmd
}

func newCmdSecFullPEM(kind string) *cobra.Command {
	var options commands.CmdFullPEM
	cmd := &cobra.Command{
		Use:   "fullpem",
		Short: "dump the x509 private key and certificate chain in PEM format",
		Long:  "A sec can contain a certificate, created by the gencert command. The private_key, certificate and certificate_chain are stored as sec keys. The fullpem command decodes the private_key and certificate_chain keys, concatenate and print the results.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
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
