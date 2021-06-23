package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

var (
	subVol = &cobra.Command{
		Use:   "vol",
		Short: "Manage volumes",
		Long: `A volume is a persistent data provider.

A volume is made of disk, fs and sync resources. It is created by a pool,
to satisfy a demand from a volume resource in a service.

Volumes and their subdirectories can be mounted inside containers.

A volume can host cfg and sec keys projections.`,
	}
	subVolEdit = &cobra.Command{
		Use:     "edit",
		Short:   "edit information about the object",
		Aliases: []string{"edi", "ed"},
	}
	subVolPrint = &cobra.Command{
		Use:     "print",
		Short:   "print information about the object",
		Aliases: []string{"prin", "pri", "pr"},
	}
)

func init() {
	var (
		cmdCreate           commands.CmdObjectCreate
		cmdDelete           commands.CmdObjectDelete
		cmdEditConfig       commands.CmdObjectEditConfig
		cmdEval             commands.CmdObjectEval
		cmdFreeze           commands.CmdObjectFreeze
		cmdGet              commands.CmdObjectGet
		cmdLs               commands.CmdObjectLs
		cmdMonitor          commands.CmdObjectMonitor
		cmdPrintConfig      commands.CmdObjectPrintConfig
		cmdPrintConfigMtime commands.CmdObjectPrintConfigMtime
		cmdPrintStatus      commands.CmdObjectPrintStatus
		cmdPrintSchedule    commands.CmdObjectPrintSchedule
		cmdProvision        commands.CmdObjectProvision
		cmdSet              commands.CmdObjectSet
		cmdStart            commands.CmdObjectStart
		cmdStatus           commands.CmdObjectStatus
		cmdStop             commands.CmdObjectStop
		cmdUnfreeze         commands.CmdObjectUnfreeze
		cmdUnprovision      commands.CmdObjectUnprovision
		cmdUnset            commands.CmdObjectUnset
	)

	kind := "vol"
	head := subVol
	subEdit := subVolEdit
	subPrint := subVolPrint
	root := rootCmd

	root.AddCommand(head)
	head.AddCommand(subEdit)
	head.AddCommand(subPrint)

	cmdCreate.Init(kind, head, &selectorFlag)
	cmdDelete.Init(kind, head, &selectorFlag)
	cmdEditConfig.Init(kind, subEdit, &selectorFlag)
	cmdEval.Init(kind, head, &selectorFlag)
	cmdFreeze.Init(kind, head, &selectorFlag)
	cmdGet.Init(kind, head, &selectorFlag)
	cmdLs.Init(kind, head, &selectorFlag)
	cmdMonitor.Init(kind, head, &selectorFlag)
	cmdPrintConfig.Init(kind, subPrint, &selectorFlag)
	cmdPrintConfigMtime.Init(kind, cmdPrintConfig.Command, &selectorFlag)
	cmdPrintStatus.Init(kind, subPrint, &selectorFlag)
	cmdPrintSchedule.Init(kind, subPrint, &selectorFlag)
	cmdProvision.Init(kind, head, &selectorFlag)
	cmdSet.Init(kind, head, &selectorFlag)
	cmdStart.Init(kind, head, &selectorFlag)
	cmdStatus.Init(kind, head, &selectorFlag)
	cmdStop.Init(kind, head, &selectorFlag)
	cmdUnfreeze.Init(kind, head, &selectorFlag)
	cmdUnprovision.Init(kind, head, &selectorFlag)
	cmdUnset.Init(kind, head, &selectorFlag)
}
