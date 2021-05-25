package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

var (
	subSvcEdit = &cobra.Command{
		Use:     "edit",
		Short:   "edit information about the object",
		Aliases: []string{"edi", "ed"},
	}

	subSvcPrint = &cobra.Command{
		Use:     "print",
		Short:   "print information about the object",
		Aliases: []string{"prin", "pri", "pr"},
	}
	subSvc = &cobra.Command{
		Use:   "svc",
		Short: "Manage services",
		Long: `Service objects subsystem.
	
A service is typically made of ip, app, container and task resources.

They can use support objects like volumes, secrets and configmaps to
isolate lifecycles or to abstract cluster-specific knowledge.
`,
	}
)

func init() {
	var (
		cmdCreate      commands.CmdObjectCreate
		cmdDelete      commands.CmdObjectDelete
		cmdEditConfig  commands.CmdObjectEditConfig
		cmdEval        commands.CmdObjectEval
		cmdFreeze      commands.CmdObjectFreeze
		cmdGet         commands.CmdObjectGet
		cmdLs          commands.CmdObjectLs
		cmdMonitor     commands.CmdObjectMonitor
		cmdPrintConfig commands.CmdObjectPrintConfig
		cmdPrintStatus commands.CmdObjectPrintStatus
		cmdSet         commands.CmdObjectSet
		cmdStart       commands.CmdObjectStart
		cmdStatus      commands.CmdObjectStatus
		cmdStop        commands.CmdObjectStop
		cmdUnfreeze    commands.CmdObjectUnfreeze
		cmdUnset       commands.CmdObjectUnset
	)

	kind := "svc"
	head := subSvc
	subEdit := subSvcEdit
	subPrint := subSvcPrint
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
	cmdPrintStatus.Init(kind, subPrint, &selectorFlag)
	cmdSet.Init(kind, head, &selectorFlag)
	cmdStart.Init(kind, head, &selectorFlag)
	cmdStatus.Init(kind, head, &selectorFlag)
	cmdStop.Init(kind, head, &selectorFlag)
	cmdUnfreeze.Init(kind, head, &selectorFlag)
	cmdUnset.Init(kind, head, &selectorFlag)
}
