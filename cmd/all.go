package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

var (
	subAll = &cobra.Command{
		Hidden: true,
		Use:    "all",
		Short:  "Manage a mix of objects",
	}
	subAllPrint = &cobra.Command{
		Use:     "print",
		Short:   "print information about the object",
		Aliases: []string{"prin", "pri", "pr"},
	}
)

func init() {
	var (
		cmdCreate           commands.CmdObjectCreate
		cmdDelete           commands.CmdObjectDelete
		cmdEdit             commands.CmdObjectEdit
		cmdEditConfig       commands.CmdObjectEditConfig
		cmdEval             commands.CmdObjectEval
		cmdFreeze           commands.CmdObjectFreeze
		cmdGet              commands.CmdObjectGet
		cmdLs               commands.CmdObjectLs
		cmdMonitor          commands.CmdObjectMonitor
		cmdPrintConfig      commands.CmdObjectPrintConfig
		cmdPrintConfigMtime commands.CmdObjectPrintConfigMtime
		cmdPrintStatus      commands.CmdObjectPrintStatus
		cmdSet              commands.CmdObjectSet
		cmdStart            commands.CmdObjectStart
		cmdStatus           commands.CmdObjectStatus
		cmdStop             commands.CmdObjectStop
		cmdUnfreeze         commands.CmdObjectUnfreeze
		cmdUnset            commands.CmdObjectUnset
	)

	kind := ""
	head := subAll
	subPrint := subAllPrint
	root := rootCmd

	root.AddCommand(head)
	head.AddCommand(subPrint)

	cmdCreate.Init(kind, head, &selectorFlag)
	cmdDelete.Init(kind, head, &selectorFlag)
	cmdEdit.Init(kind, head, &selectorFlag)
	cmdEditConfig.Init(kind, cmdEdit.Command, &selectorFlag)
	cmdEval.Init(kind, head, &selectorFlag)
	cmdFreeze.Init(kind, head, &selectorFlag)
	cmdGet.Init(kind, head, &selectorFlag)
	cmdLs.Init(kind, head, &selectorFlag)
	cmdMonitor.Init(kind, head, &selectorFlag)
	cmdPrintConfig.Init(kind, subPrint, &selectorFlag)
	cmdPrintConfigMtime.Init(kind, cmdPrintConfig.Command, &selectorFlag)
	cmdPrintStatus.Init(kind, subPrint, &selectorFlag)
	cmdSet.Init(kind, head, &selectorFlag)
	cmdStart.Init(kind, head, &selectorFlag)
	cmdStatus.Init(kind, head, &selectorFlag)
	cmdStop.Init(kind, head, &selectorFlag)
	cmdUnfreeze.Init(kind, head, &selectorFlag)
	cmdUnset.Init(kind, head, &selectorFlag)
}
