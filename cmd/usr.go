package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

func makeSubUsr() *cobra.Command {
	return &cobra.Command{
		Use:   "usr",
		Short: "Manage users",
		Long: ` A user stores the grants and credentials of user of the agent API.

User objects are not necessary with OpenID authentication, as the
grants are embedded in the trusted bearer tokens.`,
	}
}

func init() {
	var (
		cmdCreate           commands.CmdObjectCreate
		cmdDoc              commands.CmdObjectDoc
		cmdDelete           commands.CmdObjectDelete
		cmdEval             commands.CmdObjectEval
		cmdGet              commands.CmdObjectGet
		cmdLs               commands.CmdObjectLs
		cmdLogs             commands.CmdObjectLogs
		cmdMonitor          commands.CmdObjectMonitor
		cmdPrintConfig      commands.CmdObjectPrintConfig
		cmdPrintConfigMtime commands.CmdObjectPrintConfigMtime
		cmdPrintStatus      commands.CmdObjectPrintStatus
		cmdSet              commands.CmdObjectSet
		cmdStatus           commands.CmdObjectStatus
		cmdUnset            commands.CmdObjectUnset
		cmdValidateConfig   commands.CmdObjectValidateConfig

		cmdAdd     commands.CmdKeystoreAdd
		cmdChange  commands.CmdKeystoreChange
		cmdDecode  commands.CmdKeystoreDecode
		cmdKeys    commands.CmdKeystoreKeys
		cmdRemove  commands.CmdKeystoreRemove
		cmdGenCert commands.CmdSecGenCert
		cmdFullPEM commands.CmdFullPEM
		cmdPKCS    commands.CmdPKCS
	)

	kind := "usr"
	head := makeSubUsr()
	cmdEdit := newObjectEdit(kind)
	cmdEdit.AddCommand(newObjectEditConfig(kind))

	root.AddCommand(head)
	head.AddCommand(cmdEdit)

	cmdCreate.Init(kind, head, &selectorFlag)
	cmdDoc.Init(kind, head, &selectorFlag)
	cmdDelete.Init(kind, head, &selectorFlag)
	cmdEval.Init(kind, head, &selectorFlag)
	cmdFullPEM.Init(kind, head, &selectorFlag)
	cmdPKCS.Init(kind, head, &selectorFlag)
	cmdGet.Init(kind, head, &selectorFlag)
	cmdLs.Init(kind, head, &selectorFlag)
	cmdLogs.Init(kind, head, &selectorFlag)
	cmdMonitor.Init(kind, head, &selectorFlag)
	cmdSet.Init(kind, head, &selectorFlag)
	cmdStatus.Init(kind, head, &selectorFlag)
	cmdUnset.Init(kind, head, &selectorFlag)

	cmdAdd.Init(kind, head, &selectorFlag)
	cmdChange.Init(kind, head, &selectorFlag)
	cmdDecode.Init(kind, head, &selectorFlag)
	cmdKeys.Init(kind, head, &selectorFlag)
	cmdRemove.Init(kind, head, &selectorFlag)
	cmdGenCert.Init(kind, head, &selectorFlag)

	if sub := makeSubPrint(); sub != nil {
		head.AddCommand(sub)
		cmdPrintConfig.Init(kind, sub, &selectorFlag)
		cmdPrintConfigMtime.Init(kind, cmdPrintConfig.Command, &selectorFlag)
		cmdPrintStatus.Init(kind, sub, &selectorFlag)
	}

	if sub := makeSubValidate(); sub != nil {
		head.AddCommand(sub)
		cmdValidateConfig.Init(kind, sub, &selectorFlag)
	}
}
