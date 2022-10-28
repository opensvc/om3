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
		cmdAbort                     commands.CmdObjectAbort
		cmdClear                     commands.CmdObjectClear
		cmdCreate                    commands.CmdObjectCreate
		cmdComplianceAttachModuleset commands.CmdObjectComplianceAttachModuleset
		cmdComplianceDetachModuleset commands.CmdObjectComplianceDetachModuleset
		cmdComplianceAttachRuleset   commands.CmdObjectComplianceAttachRuleset
		cmdComplianceDetachRuleset   commands.CmdObjectComplianceDetachRuleset
		cmdComplianceAuto            commands.CmdObjectComplianceAuto
		cmdComplianceCheck           commands.CmdObjectComplianceCheck
		cmdComplianceFix             commands.CmdObjectComplianceFix
		cmdComplianceFixable         commands.CmdObjectComplianceFixable
		cmdComplianceShowRuleset     commands.CmdObjectComplianceShowRuleset
		cmdComplianceShowModuleset   commands.CmdObjectComplianceShowModuleset
		cmdComplianceListModules     commands.CmdObjectComplianceListModules
		cmdComplianceListModuleset   commands.CmdObjectComplianceListModuleset
		cmdComplianceListRuleset     commands.CmdObjectComplianceListRuleset
		cmdComplianceEnv             commands.CmdObjectComplianceEnv
		cmdDelete                    commands.CmdObjectDelete
		cmdDoc                       commands.CmdObjectDoc
		cmdEval                      commands.CmdObjectEval
		cmdEnter                     commands.CmdObjectEnter
		cmdFreeze                    commands.CmdObjectFreeze
		cmdGet                       commands.CmdObjectGet
		cmdLogs                      commands.CmdObjectLogs
		cmdLs                        commands.CmdObjectLs
		cmdMonitor                   commands.CmdObjectMonitor
		cmdPrintConfig               commands.CmdObjectPrintConfig
		cmdPrintConfigMtime          commands.CmdObjectPrintConfigMtime
		cmdPrintDevices              commands.CmdObjectPrintDevices
		cmdPrintStatus               commands.CmdObjectPrintStatus
		cmdPrintSchedule             commands.CmdObjectPrintSchedule
		cmdProvision                 commands.CmdObjectProvision
		cmdPRStart                   commands.CmdObjectPRStart
		cmdPRStop                    commands.CmdObjectPRStop
		cmdPurge                     commands.CmdObjectPurge
		cmdPushResInfo               commands.CmdObjectPushResInfo
		cmdRestart                   commands.CmdObjectRestart
		cmdRun                       commands.CmdObjectRun
		cmdSet                       commands.CmdObjectSet
		cmdSetProvisioned            commands.CmdObjectSetProvisioned
		cmdSetUnprovisioned          commands.CmdObjectSetUnprovisioned
		cmdStart                     commands.CmdObjectStart
		cmdStatus                    commands.CmdObjectStatus
		cmdStop                      commands.CmdObjectStop
		cmdSyncResync                commands.CmdObjectSyncResync
		cmdThaw                      commands.CmdObjectThaw
		cmdUnfreeze                  commands.CmdObjectUnfreeze
		cmdUnprovision               commands.CmdObjectUnprovision
		cmdUnset                     commands.CmdObjectUnset
		cmdValidateConfig            commands.CmdObjectValidateConfig
	)

	kind := "svc"
	if head := makeSubSVC(); head != nil {
		root.AddCommand(head)
		cmdEdit := newObjectEdit(kind)
		cmdEdit.AddCommand(newObjectEditConfig(kind))

		head.AddCommand(cmdEdit)

		cmdAbort.Init(kind, head, &selectorFlag)
		cmdClear.Init(kind, head, &selectorFlag)
		cmdCreate.Init(kind, head, &selectorFlag)
		cmdDoc.Init(kind, head, &selectorFlag)
		cmdDelete.Init(kind, head, &selectorFlag)
		cmdEval.Init(kind, head, &selectorFlag)
		cmdEnter.Init(kind, head, &selectorFlag)
		cmdFreeze.Init(kind, head, &selectorFlag)
		cmdGet.Init(kind, head, &selectorFlag)
		cmdLs.Init(kind, head, &selectorFlag)
		cmdLogs.Init(kind, head, &selectorFlag)
		cmdMonitor.Init(kind, head, &selectorFlag)
		cmdProvision.Init(kind, head, &selectorFlag)
		cmdPRStart.Init(kind, head, &selectorFlag)
		cmdPRStop.Init(kind, head, &selectorFlag)
		cmdPurge.Init(kind, head, &selectorFlag)
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

		if sub := makeSubCompliance(); sub != nil {
			head.AddCommand(sub)
			cmdComplianceEnv.Init(kind, sub, &selectorFlag)
			cmdComplianceAuto.Init(kind, sub, &selectorFlag)
			cmdComplianceCheck.Init(kind, sub, &selectorFlag)
			cmdComplianceFix.Init(kind, sub, &selectorFlag)
			cmdComplianceFixable.Init(kind, sub, &selectorFlag)
			if subsub := makeSubComplianceAttach(); sub != nil {
				sub.AddCommand(subsub)
				cmdComplianceAttachModuleset.Init(kind, subsub, &selectorFlag)
				cmdComplianceAttachRuleset.Init(kind, subsub, &selectorFlag)
			}
			if subsub := makeSubComplianceDetach(); subsub != nil {
				sub.AddCommand(subsub)
				cmdComplianceDetachModuleset.Init(kind, subsub, &selectorFlag)
				cmdComplianceDetachRuleset.Init(kind, subsub, &selectorFlag)
			}
			if subsub := makeSubComplianceShow(); subsub != nil {
				sub.AddCommand(subsub)
				cmdComplianceShowRuleset.Init(kind, subsub, &selectorFlag)
				cmdComplianceShowModuleset.Init(kind, subsub, &selectorFlag)
			}
			if subsub := makeSubComplianceList(); subsub != nil {
				sub.AddCommand(subsub)
				cmdComplianceListModules.Init(kind, subsub, &selectorFlag)
				cmdComplianceListModuleset.Init(kind, subsub, &selectorFlag)
				cmdComplianceListRuleset.Init(kind, subsub, &selectorFlag)
			}
		}

		if sub := makeSubPrint(); sub != nil {
			head.AddCommand(sub)
			cmdPrintConfig.Init(kind, sub, &selectorFlag)
			cmdPrintConfigMtime.Init(kind, cmdPrintConfig.Command, &selectorFlag)
			cmdPrintDevices.Init(kind, sub, &selectorFlag)
			cmdPrintStatus.Init(kind, sub, &selectorFlag)
			cmdPrintSchedule.Init(kind, sub, &selectorFlag)
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
}
