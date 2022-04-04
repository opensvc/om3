package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

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

func makeSubSync() *cobra.Command {
	return &cobra.Command{
		Use:     "sync",
		Short:   "group data synchronization commands",
		Aliases: []string{"syn", "sy"},
	}
}

func makeSubEdit() *cobra.Command {
	return &cobra.Command{
		Use:     "edit",
		Short:   "edit information about the object",
		Aliases: []string{"edi", "ed"},
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

func init() {
	var (
		cmdCreate           commands.CmdObjectCreate
		cmdDelete           commands.CmdObjectDelete
		cmdDoc              commands.CmdObjectDoc
		cmdEdit             commands.CmdObjectEdit
		cmdEditConfig       commands.CmdObjectEditConfig
		cmdEnter            commands.CmdObjectEnter
		cmdEval             commands.CmdObjectEval
		cmdFreeze           commands.CmdObjectFreeze
		cmdGet              commands.CmdObjectGet
		cmdLs               commands.CmdObjectLs
		cmdMonitor          commands.CmdObjectMonitor
		cmdPrintConfig      commands.CmdObjectPrintConfig
		cmdPrintConfigMtime commands.CmdObjectPrintConfigMtime
		cmdPrintDevices     commands.CmdObjectPrintDevices
		cmdPrintSchedule    commands.CmdObjectPrintSchedule
		cmdPrintStatus      commands.CmdObjectPrintStatus
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
	)

	kind := ""
	head := makeHead()
	root.AddCommand(head)
	cmdCreate.Init(kind, head, &selectorFlag)
	cmdDelete.Init(kind, head, &selectorFlag)
	cmdDoc.Init(kind, head, &selectorFlag)
	cmdEdit.Init(kind, head, &selectorFlag)
	cmdEditConfig.Init(kind, cmdEdit.Command, &selectorFlag)
	cmdEval.Init(kind, head, &selectorFlag)
	cmdEnter.Init(kind, head, &selectorFlag)
	cmdFreeze.Init(kind, head, &selectorFlag)
	cmdGet.Init(kind, head, &selectorFlag)
	cmdLs.Init(kind, head, &selectorFlag)
	cmdMonitor.Init(kind, head, &selectorFlag)
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

	if sub := makeSubSync(); sub != nil {
		head.AddCommand(sub)
		cmdSyncResync.Init(kind, sub, &selectorFlag)
	}
}
