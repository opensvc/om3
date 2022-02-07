package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

func makeSubSVC() *cobra.Command {
	return &cobra.Command{
		Use:   "svc",
		Short: "Manage services",
		Long: `Service objects subsystem.
	
A service is typically made of ip, app, container and task resources.

They can use support objects like volumes, secrets and configmaps to
isolate lifecycles or to abstract cluster-specific knowledge.
`,
	}
}

func init() {
	var (
		cmdCreate           commands.CmdObjectCreate
		cmdDelete           commands.CmdObjectDelete
		cmdDoc              commands.CmdObjectDoc
		cmdEditConfig       commands.CmdObjectEditConfig
		cmdEval             commands.CmdObjectEval
		cmdEnter            commands.CmdObjectEnter
		cmdFreeze           commands.CmdObjectFreeze
		cmdGet              commands.CmdObjectGet
		cmdLs               commands.CmdObjectLs
		cmdMonitor          commands.CmdObjectMonitor
		cmdPrintConfig      commands.CmdObjectPrintConfig
		cmdPrintConfigMtime commands.CmdObjectPrintConfigMtime
		cmdPrintDevices     commands.CmdObjectPrintDevices
		cmdPrintStatus      commands.CmdObjectPrintStatus
		cmdPrintSchedule    commands.CmdObjectPrintSchedule
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

	kind := "svc"
	if head := makeSubSVC(); head != nil {
		root.AddCommand(head)
		cmdCreate.Init(kind, head, &selectorFlag)
		cmdDoc.Init(kind, head, &selectorFlag)
		cmdDelete.Init(kind, head, &selectorFlag)
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

		if sub := makeSubEdit(); sub != nil {
			head.AddCommand(sub)
			cmdEditConfig.Init(kind, sub, &selectorFlag)
		}

		if sub := makeSubPrint(); sub != nil {
			head.AddCommand(sub)
			cmdPrintConfig.Init(kind, sub, &selectorFlag)
			cmdPrintConfigMtime.Init(kind, cmdPrintConfig.Command, &selectorFlag)
			cmdPrintDevices.Init(kind, sub, &selectorFlag)
			cmdPrintStatus.Init(kind, sub, &selectorFlag)
			cmdPrintSchedule.Init(kind, sub, &selectorFlag)
		}

		if sub := makeSubSync(); sub != nil {
			head.AddCommand(sub)
			cmdSyncResync.Init(kind, sub, &selectorFlag)
		}
	}
}
