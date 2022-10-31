package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
	"opensvc.com/opensvc/core/monitor"
)

func newCmdAll() *cobra.Command {
	return &cobra.Command{
		Hidden: false,
		Use:    "all",
		Short:  "Manage a mix of objects, tentatively exposing all commands.",
	}
}

func newCmdCcfg() *cobra.Command {
	return &cobra.Command{
		Use:   "ccfg",
		Short: "Manage the cluster shared configuration.",
		Long: ` The cluster nodes merge their private configuration
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
		Short: "Manage configmaps.",
		Long: ` A configmap is an unencrypted key-value store.

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
		Short: "Manage secrets.",
		Long: `	A secret is an encrypted key-value store.

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
		Short: "Manage services.",
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
		Short: "Manage volumes.",
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
		Short: "Manage users.",
		Long: ` A user stores the grants and credentials of user of the agent API.

User objects are not necessary with OpenID authentication, as the
grants are embedded in the trusted bearer tokens.`,
	}
}

func newCmdArrayLs() *cobra.Command {
	var options commands.CmdArrayLs
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List the cluster-managed storage arrays.",
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
		Short: "Show the daemon relay clients and last data update time.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flagSet := cmd.Flags()
	addFlagsGlobal(flagSet, &options.OptsGlobal)
	addFlagRelay(flagSet, &options.Relays)
	return cmd
}

func newCmdDaemonRestart() *cobra.Command {
	var options commands.CmdDaemonRestart
	cmd := &cobra.Command{
		Use:     "restart",
		Short:   "Start the daemon or a daemon thread pointed by '--thread-id'.",
		Aliases: []string{"restart"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flagSet := cmd.Flags()
	addFlagsGlobal(flagSet, &options.OptsGlobal)
	addFlagForeground(flagSet, &options.Foreground)
	addFlagDebug(flagSet, &options.Debug)
	return cmd
}

func newCmdDaemonRunning() *cobra.Command {
	var options commands.CmdDaemonRunning
	cmd := &cobra.Command{
		Use:   "running",
		Short: "Return with code 0 if the daemon is running, else return with code 1",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flagSet := cmd.Flags()
	addFlagsGlobal(flagSet, &options.OptsGlobal)
	return cmd
}

func newCmdDaemonStart() *cobra.Command {
	var options commands.CmdDaemonStart
	cmd := &cobra.Command{
		Use:     "start",
		Short:   "Start the daemon or a daemon thread pointed by '--thread-id'.",
		Aliases: []string{"star"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flagSet := cmd.Flags()
	addFlagsGlobal(flagSet, &options.OptsGlobal)
	addFlagDebug(flagSet, &options.Debug)
	addFlagForeground(flagSet, &options.Foreground)
	addFlagCpuProfile(flagSet, &options.CpuProfile)
	return cmd
}

func newCmdDaemonStatus() *cobra.Command {
	var options commands.CmdDaemonStatus
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Print the cluster status.",
		Long:    monitor.CmdLong,
		Aliases: []string{"statu"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flagSet := cmd.Flags()
	addFlagsGlobal(flagSet, &options.OptsGlobal)
	addFlagWatch(flagSet, &options.Watch)
	return cmd
}

func newCmdDaemonStats() *cobra.Command {
	var options commands.CmdDaemonStats
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Print the resource usage statistics.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flagSet := cmd.Flags()
	addFlagsGlobal(flagSet, &options.OptsGlobal)
	return cmd
}

func newCmdDaemonStop() *cobra.Command {
	var options commands.CmdDaemonStop
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the daemon.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flagSet := cmd.Flags()
	addFlagsGlobal(flagSet, &options.OptsGlobal)
	return cmd
}

func newCmdKeystoreAdd(kind string) *cobra.Command {
	var options commands.CmdKeystoreAdd
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add new keys.",
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
		Short: "Change existing keys value.",
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
		Short: "Decode a keystore object key value.",
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
		Short: "Install keys as files in their projected volume locations.",
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
		Short: "List the object key names.",
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
		Short: "Remove a object key.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagKey(flags, &options.Key)
	return cmd
}

func newCmdSecFullPEM(kind string) *cobra.Command {
	var options commands.CmdFullPEM
	cmd := &cobra.Command{
		Use:   "fullpem",
		Short: "Dump the private_key and certificate_chain in PEM format.",
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
		Short: "Create or replace a x509 certificate stored as a keyset.",
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
		Short: "Dump the private_key and certificate_chain in PKCS#12 format (bytes).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdNetworkLs() *cobra.Command {
	var options commands.CmdNetworkLs
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List the cluster networks.",
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
		Short:   "Configure the cluster networks on the node.",
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
		Short:   "Show the cluster networks usage.",
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

func newCmdNodeChecks() *cobra.Command {
	var options commands.CmdNodeChecks
	cmd := &cobra.Command{
		Use:     "checks",
		Short:   "Run the check drivers, push and print the instances.",
		Aliases: []string{"check", "chec", "che", "ch"},
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
		Short:   "Attach compliance moduleset to this node.",
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

func newCmdNodeComplianceAttachRuleset() *cobra.Command {
	var options commands.CmdNodeComplianceAttachRuleset
	cmd := &cobra.Command{
		Use:     "ruleset",
		Short:   "Attach compliance ruleset to this node.",
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

func newCmdNodeComplianceAuto() *cobra.Command {
	var options commands.CmdNodeComplianceAuto
	cmd := &cobra.Command{
		Use:   "auto",
		Short: "Run compliance fixes on autofix modules, checks on other modules",
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
		Short:   "Run compliance checks.",
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
		Short: "Run compliance fixes.",
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
		Short:   "Run compliance fixable-tests.",
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
		Short:   "Detach compliance moduleset to this node.",
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
		Short:   "Detach compliance ruleset from this node.",
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
		Short:   "Show the environment variables set during a compliance module run.",
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
		Short:   "List modules available on this node.",
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
		Short:   "List compliance moduleset available to this node.",
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
		Short:   "List compliance ruleset available to this node.",
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
		Short:   "Show compliance moduleset and modules attached to this node.",
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
		Short:   "Show compliance rules applying to this node.",
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
		Short: "Delete a configuration section.",
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
		Short: "Print the documentation of the selected keywords.",
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

func newCmdNodeDrivers() *cobra.Command {
	var options commands.CmdNodeDrivers
	cmd := &cobra.Command{
		Use:     "drivers",
		Short:   "List builtin drivers.",
		Aliases: []string{"driver", "drive", "driv", "drv", "dr"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdNodeEditConfig() *cobra.Command {
	var options commands.CmdNodeEditConfig
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "Edit the node configuration.",
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
		Short: "Evaluate a configuration key value.",
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
		Short:   "Print the node event stream.",
		Aliases: []string{"eve", "even", "event"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdNodeFreeze() *cobra.Command {
	var options commands.CmdNodeFreeze
	cmd := &cobra.Command{
		Use:   "freeze",
		Short: "Freeze the node.",
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
		Short: "Get a configuration key value.",
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
		Short:   "Filter and format logs.",
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
		Short: "List the cluster nodes.",
		Long:  "The list can be filtered using the --node selector. This command can be used to validate node selector expressions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	return cmd
}

func newCmdNodePrintCapabilities() *cobra.Command {
	var options commands.CmdNodePrintCapabilities
	cmd := &cobra.Command{
		Use:     "capabilities",
		Short:   "Print the node capabilities.",
		Aliases: []string{"capa", "cap", "ca", "caps"},
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
		Short:   "Print the node configuration.",
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
		Short:   "Print selected objects scheduling table.",
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
		Short:   "Run the node discovery, push and print the result.",
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
		Short:   "Run the disk discovery, push and print the result.",
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
		Short:   "Run the node installed patches discovery, push and print the result.",
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
		Short:   "Run the node installed packages discovery, push and print the result.",
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
		Short:   "Obtain a registration id from the collector. This is is then used to authenticate the node in collector communications.",
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

func newCmdNodeScanCapabilities() *cobra.Command {
	var options commands.CmdNodeScanCapabilities
	cmd := &cobra.Command{
		Use:     "capabilities",
		Short:   "Scan the node for capabilities.",
		Aliases: []string{"capa", "caps", "cap", "ca", "c"},
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

func newCmdNodeSet() *cobra.Command {
	var options commands.CmdNodeSet
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set a configuration key value.",
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
		Short:   "Push system report to the collector for archiving and diff analysis. The --force option resend all monitored files and outputs to the collector instead of only those that changed since the last sysreport.",
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
		Short:   "Unfreeze the node.",
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
		Short:   "Unfreeze the node.",
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
		Short: "Unset a configuration key.",
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
		Short:   "Verify the node configuration syntax is valid.",
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
		Short: "Abort the running orchestration.",
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
		Short: "Clear errors in the monitor state.",
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
		Short:   "Print information about the object.",
		Aliases: []string{"prin", "pri", "pr"},
	}
}

func newCmdObjectPush(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "push",
		Short:   "Push information about the object to the collector.",
		Aliases: []string{"push", "pus", "pu"},
	}
}

func newCmdObjectValidate(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "validate",
		Short:   "Validation command group.",
		Aliases: []string{"validat", "valida", "valid", "vali", "val"},
	}
}

func newCmdObjectSync(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "sync",
		Short:   "Data synchronization command group.",
		Aliases: []string{"syn", "sy"},
	}
}

func newCmdObjectCompliance(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "compliance",
		Short:   "Node configuration expectations analysis and application.",
		Aliases: []string{"compli", "comp", "com", "co"},
	}
}

func newCmdObjectComplianceAttach(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "attach",
		Short:   "Attach modulesets and rulesets to the node.",
		Aliases: []string{"attac", "atta", "att", "at"},
	}
}

func newCmdObjectComplianceDetach(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "detach",
		Short:   "Detach modulesets and rulesets from the node.",
		Aliases: []string{"detac", "deta", "det", "de"},
	}
}

func newCmdObjectComplianceList(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List modules, modulesets and rulesets available.",
		Aliases: []string{"lis", "li", "ls", "l"},
	}
}

func newCmdObjectComplianceShow(kind string) *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Short:   "Show states: current moduleset and ruleset attachments, modules last check.",
		Aliases: []string{"sho", "sh", "s"},
	}
}

func newCmdObjectEdit(kind string) *cobra.Command {
	var optionsGlobal commands.OptsGlobal
	var optionsConfig commands.CmdObjectEditConfig
	var optionsKey commands.CmdObjectEditKey
	cmd := &cobra.Command{
		Use:     "edit",
		Short:   "Edit object configuration or keystore key.",
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
		Short:   "Edit selected object and instance configuration.",
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
		Short:   "Attach compliance moduleset to this object.",
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
		Short:   "Attach compliance ruleset to this object.",
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

func newCmdObjectComplianceAuto(kind string) *cobra.Command {
	var options commands.CmdObjectComplianceAuto
	cmd := &cobra.Command{
		Use:   "auto",
		Short: "Run compliance fixes on autofix modules, checks on other modules.",
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
		Short:   "Run compliance checks.",
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
		Short: "Run compliance fixes.",
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
		Short:   "Run compliance fixable-tests.",
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
		Short:   "Detach compliance moduleset to this object.",
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
		Short:   "Detach compliance ruleset from this object.",
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
		Short:   "Show the environment variables set during a compliance module run.",
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
		Short:   "List modules available on this object.",
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
		Short:   "List compliance moduleset available to this object.",
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
		Short:   "List compliance ruleset available to this object.",
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
		Short:   "Show compliance moduleset and modules attached to this object.",
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
		Short:   "Show compliance rules applying to this object.",
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
		Short: "Create new objects.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsLock(flags, &options.OptsLock)
	addFlagsResourceSelector(flags, &options.OptsResourceSelector)
	addFlagsTo(flags, &options.OptTo)
	addFlagTemplate(flags, &options.Template)
	addFlagConfig(flags, &options.Config)
	addFlagKeywords(flags, &options.Keywords)
	addFlagEnv(flags, &options.Env)
	addFlagInteractive(flags, &options.Interactive)
	addFlagProvision(flags, &options.Provision)
	addFlagCreateRestore(flags, &options.Restore)
	addFlagCreateForce(flags, &options.Force)
	addFlagCreateNamespace(flags, &options.Namespace)
	return cmd
}

func newCmdObjectDelete(kind string) *cobra.Command {
	var options commands.CmdObjectDelete
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete the object, an instance or a configuration section.",
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
		Short: "Print the documentation of the selected keywords.",
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
		Short: "Open a shell in a container resource.",
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
		Short: "Evaluate a configuration key value.",
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
		Short: "Freeze the selected objects.",
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
		Short: "Get a configuration key value.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagKeyword(flags, &options.Keyword)
	return cmd
}

func newCmdObjectLogs(kind string) *cobra.Command {
	var options commands.CmdObjectLogs
	cmd := &cobra.Command{
		Use:     "logs",
		Aliases: []string{"logs", "log", "lo"},
		Short:   "Filter and format logs.",
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
		Short: "Print the selected objects path.",
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
		Short:   "Print selected service and instance status summary.",
		Long:    monitor.CmdLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagWatch(flags, &options.Watch)
	return cmd
}

func newCmdObjectPrintConfig(kind string) *cobra.Command {
	var options commands.CmdObjectPrintConfig
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "Print selected object and instance configuration.",
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
		Short:   "Print selected object and instance configuration file modification time.",
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
		Short:   "Print selected objects and resources exposed, used, base and claimed block devices.",
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
		Short:   "Print selected objects scheduling table.",
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
		Short:   "Print selected service and instance status.",
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
		Short:   "Allocate new resources.",
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
		Short: "Preempt devices exclusive write access reservation.",
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
		Short: "Release devices exclusive write access reservation.",
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
		Short: "Unprovision and delete.",
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
		Short:   "Push resource info key/val pairs.",
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
		Short: "Restart the selected objects or resources.",
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

func newCmdObjectRun(kind string) *cobra.Command {
	var options commands.CmdObjectRun
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run tasks now.",
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
		Short: "Set a configuration key value.",
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
		Short:   "Set the resources as provisioned.",
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
		Short:   "Set the resources as unprovisioned.",
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

func newCmdObjectStart(kind string) *cobra.Command {
	var options commands.CmdObjectStart
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the selected objects.",
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
		Short:   "Print selected service and instance status.",
		Long: `Resources Flags:

(1) R   Running,           . Not Running
(2) M   Monitored,         . Not Monitored
(3) D   Disabled,          . Enabled
(4) O   Optional,          . Not Optional
(5) E   Encap,             . Not Encap
(6) P   Not Provisioned,   . Provisioned,       p Provisioned Mixed,  / Provisioned undef or n/a
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

func newCmdObjectStop(kind string) *cobra.Command {
	var options commands.CmdObjectStop
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the selected objects.",
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

func newCmdObjectSyncResync(kind string) *cobra.Command {
	var options commands.CmdObjectSyncResync
	cmd := &cobra.Command{
		Use:   "resync",
		Short: "Restore optimal synchronization.",
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

func newCmdObjectUnfreeze(kind string) *cobra.Command {
	var options commands.CmdObjectUnfreeze
	cmd := &cobra.Command{
		Use:   "unfreeze",
		Short: "Unfreeze the selected objects.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	addFlagsAsync(flags, &options.OptsAsync)
	return cmd
}

func newCmdObjectThaw(kind string) *cobra.Command {
	var options commands.CmdObjectUnfreeze
	cmd := &cobra.Command{
		Use:    "thaw",
		Hidden: true,
		Short:  "Unfreeze the selected objects.",
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
		Short:   "Free resources (danger).",
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
		Short: "Unset a configuration key.",
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
		Short:   "Verify the object configuration syntax is correct.",
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
		Short: "List the cluster pools.",
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
		Short:   "Show the cluster pools usage.",
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
