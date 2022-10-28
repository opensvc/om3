package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"opensvc.com/opensvc/core/commands"
)

func addFlagsGlobal(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	flagSet.StringVar(&p.Color, "color", "auto", "Output colorization yes|no|auto")
	flagSet.StringVar(&p.Format, "format", "auto", "Output format json|flat|auto")
	flagSet.StringVar(&p.Server, "server", "", "URI of the opensvc api server. scheme raw|https")
	flagSet.BoolVar(&p.Local, "local", false, "Inline action on local instance")
	flagSet.StringVar(&p.NodeSelector, "node", "", "Execute on a list of nodes")
	flagSet.StringVarP(&p.ObjectSelector, "service", "s", "", "Execute on a list of objects")
}

func addFlagDiscard(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "discard", false, "Discard the stashed, invalid, configuration file leftover of a previous execution")
}

func addFlagRecover(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "recover", false, "Recover the stashed, invalid, configuration file leftover of a previous execution")
}

func addFlagRelay(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "relay", "", "The name of the relay to query. If not specified, all known relays are queried.")
}

func addFlagKey(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVar(p, "key", "", "A keystore key name")
}

func makeHead() *cobra.Command {
	return &cobra.Command{
		Hidden: false,
		Use:    "all",
		Short:  "manage a mix of objects, tentatively exposing all commands",
	}
}

func makeSubPrint() *cobra.Command {
	return &cobra.Command{
		Use:     "print",
		Short:   "print information about the object",
		Aliases: []string{"prin", "pri", "pr"},
	}
}

func makeSubPush() *cobra.Command {
	return &cobra.Command{
		Use:     "push",
		Short:   "push information about the object to the collector",
		Aliases: []string{"push", "pus", "pu"},
	}
}

func makeSubValidate() *cobra.Command {
	return &cobra.Command{
		Use:     "validate",
		Short:   "validation command group",
		Aliases: []string{"validat", "valida", "valid", "vali", "val"},
	}
}

func makeSubSync() *cobra.Command {
	return &cobra.Command{
		Use:     "sync",
		Short:   "data synchronization command group",
		Aliases: []string{"syn", "sy"},
	}
}

func makeSubCompliance() *cobra.Command {
	return &cobra.Command{
		Use:     "compliance",
		Short:   "node configuration expectations analysis and application",
		Aliases: []string{"compli", "comp", "com", "co"},
	}
}

func makeSubComplianceAttach() *cobra.Command {
	return &cobra.Command{
		Use:     "attach",
		Short:   "attach modulesets and rulesets to the node.",
		Aliases: []string{"attac", "atta", "att", "at"},
	}
}

func makeSubComplianceDetach() *cobra.Command {
	return &cobra.Command{
		Use:     "detach",
		Short:   "detach modulesets and rulesets from the node.",
		Aliases: []string{"detac", "deta", "det", "de"},
	}
}

func makeSubComplianceList() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "list modules, modulesets and rulesets available",
		Aliases: []string{"lis", "li", "ls", "l"},
	}
}

func makeSubComplianceShow() *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Short:   "show states: current moduleset and ruleset attachments, modules last check",
		Aliases: []string{"sho", "sh", "s"},
	}
}

func newObjectEdit(kind string) *cobra.Command {
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
	flagSet := cmd.Flags()
	addFlagsGlobal(flagSet, &optionsGlobal)
	addFlagDiscard(flagSet, &optionsConfig.Discard)
	addFlagRecover(flagSet, &optionsConfig.Recover)
	addFlagKey(flagSet, &optionsKey.Key)
	cmd.MarkFlagsMutuallyExclusive("discard", "recover")
	cmd.MarkFlagsMutuallyExclusive("discard", "key")
	cmd.MarkFlagsMutuallyExclusive("recover", "key")
	return cmd
}

func newObjectEditConfig(kind string) *cobra.Command {
	var options commands.CmdObjectEditConfig
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "edit selected object and instance configuration",
		Aliases: []string{"conf", "c", "cf", "cfg"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selectorFlag, kind)
		},
	}
	flagSet := cmd.Flags()
	addFlagsGlobal(flagSet, &options.OptsGlobal)
	addFlagDiscard(flagSet, &options.Discard)
	addFlagRecover(flagSet, &options.Recover)
	cmd.MarkFlagsMutuallyExclusive("discard", "recover")
	return cmd
}

func init() {
	var (
		cmdCreate           commands.CmdObjectCreate
		cmdDelete           commands.CmdObjectDelete
		cmdDoc              commands.CmdObjectDoc
		cmdEnter            commands.CmdObjectEnter
		cmdEval             commands.CmdObjectEval
		cmdFreeze           commands.CmdObjectFreeze
		cmdGet              commands.CmdObjectGet
		cmdLogs             commands.CmdObjectLogs
		cmdLs               commands.CmdObjectLs
		cmdMonitor          commands.CmdObjectMonitor
		cmdPrintConfig      commands.CmdObjectPrintConfig
		cmdPrintConfigMtime commands.CmdObjectPrintConfigMtime
		cmdPrintDevices     commands.CmdObjectPrintDevices
		cmdPrintSchedule    commands.CmdObjectPrintSchedule
		cmdPrintStatus      commands.CmdObjectPrintStatus
		cmdPurge            commands.CmdObjectPurge
		cmdPushResInfo      commands.CmdObjectPushResInfo
		cmdProvision        commands.CmdObjectProvision
		cmdRestart          commands.CmdObjectRestart
		cmdRun              commands.CmdObjectRun
		cmdSet              commands.CmdObjectSet
		cmdSetProvisioned   commands.CmdObjectSetProvisioned
		cmdSetUnprovisioned commands.CmdObjectSetUnprovisioned
		cmdStart            commands.CmdObjectStart
		cmdStatus           commands.CmdObjectStatus
		cmdStop             commands.CmdObjectStop
		cmdSyncResync       commands.CmdObjectSyncResync
		cmdThaw             commands.CmdObjectThaw
		cmdUnfreeze         commands.CmdObjectUnfreeze
		cmdUnprovision      commands.CmdObjectUnprovision
		cmdUnset            commands.CmdObjectUnset
		cmdValidateConfig   commands.CmdObjectValidateConfig
	)

	kind := ""
	head := makeHead()

	cmdEdit := newObjectEdit(kind)
	cmdEdit.AddCommand(newObjectEditConfig(kind))

	root.AddCommand(head)
	head.AddCommand(cmdEdit)
	cmdCreate.Init(kind, head, &selectorFlag)
	cmdDelete.Init(kind, head, &selectorFlag)
	cmdDoc.Init(kind, head, &selectorFlag)
	cmdEval.Init(kind, head, &selectorFlag)
	cmdEnter.Init(kind, head, &selectorFlag)
	cmdFreeze.Init(kind, head, &selectorFlag)
	cmdGet.Init(kind, head, &selectorFlag)
	cmdLogs.Init(kind, head, &selectorFlag)
	cmdLs.Init(kind, head, &selectorFlag)
	cmdMonitor.Init(kind, head, &selectorFlag)
	cmdPurge.Init(kind, head, &selectorFlag)
	cmdProvision.Init(kind, head, &selectorFlag)
	cmdRestart.Init(kind, head, &selectorFlag)
	cmdRun.Init(kind, head, &selectorFlag)
	cmdSet.Init(kind, head, &selectorFlag)
	cmdSetProvisioned.Init(kind, cmdSet.Command, &selectorFlag)
	cmdSetUnprovisioned.Init(kind, cmdSet.Command, &selectorFlag)
	cmdStart.Init(kind, head, &selectorFlag)
	cmdStatus.Init(kind, head, &selectorFlag)
	cmdStop.Init(kind, head, &selectorFlag)
	cmdThaw.Init(kind, head, &selectorFlag)
	cmdUnfreeze.Init(kind, head, &selectorFlag)
	cmdUnprovision.Init(kind, head, &selectorFlag)
	cmdUnset.Init(kind, head, &selectorFlag)

	if sub := makeSubPrint(); sub != nil {
		head.AddCommand(sub)
		cmdPrintConfig.Init(kind, sub, &selectorFlag)
		cmdPrintConfigMtime.Init(kind, cmdPrintConfig.Command, &selectorFlag)
		cmdPrintDevices.Init(kind, sub, &selectorFlag)
		cmdPrintSchedule.Init(kind, sub, &selectorFlag)
		cmdPrintStatus.Init(kind, sub, &selectorFlag)
	}

	if sub := makeSubPush(); sub != nil {
		head.AddCommand(sub)
		cmdPushResInfo.Init(kind, sub, &selectorFlag)
	}

	if sub := makeSubSync(); sub != nil {
		head.AddCommand(sub)
		cmdSyncResync.Init(kind, sub, &selectorFlag)
	}

	if sub := makeSubValidate(); sub != nil {
		head.AddCommand(sub)
		cmdValidateConfig.Init(kind, sub, &selectorFlag)
	}
}
