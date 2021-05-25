package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

var (
	subSec = &cobra.Command{
		Use:   "sec",
		Short: "Manage secrets",
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
	subSecPrint = &cobra.Command{
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
		cmdGet              commands.CmdObjectGet
		cmdLs               commands.CmdObjectLs
		cmdMonitor          commands.CmdObjectMonitor
		cmdPrintConfig      commands.CmdObjectPrintConfig
		cmdPrintConfigMtime commands.CmdObjectPrintConfigMtime
		cmdPrintStatus      commands.CmdObjectPrintStatus
		cmdSet              commands.CmdObjectSet
		cmdStatus           commands.CmdObjectStatus
		cmdUnset            commands.CmdObjectUnset

		cmdAdd     commands.CmdKeystoreAdd
		cmdChange  commands.CmdKeystoreChange
		cmdDecode  commands.CmdKeystoreDecode
		cmdKeys    commands.CmdKeystoreKeys
		cmdRemove  commands.CmdKeystoreRemove
		cmdGenCert commands.CmdSecGenCert
	)

	kind := "sec"
	head := subSec
	subPrint := subSecPrint
	root := rootCmd

	root.AddCommand(head)
	head.AddCommand(subPrint)

	cmdAdd.Init(kind, head, &selectorFlag)
	cmdChange.Init(kind, head, &selectorFlag)
	cmdCreate.Init(kind, head, &selectorFlag)
	cmdDelete.Init(kind, head, &selectorFlag)
	cmdDecode.Init(kind, head, &selectorFlag)
	cmdEdit.Init(kind, head, &selectorFlag)
	cmdEditConfig.Init(kind, cmdEdit.Command, &selectorFlag)
	cmdEval.Init(kind, head, &selectorFlag)
	cmdGenCert.Init(kind, head, &selectorFlag)
	cmdGet.Init(kind, head, &selectorFlag)
	cmdKeys.Init(kind, head, &selectorFlag)
	cmdLs.Init(kind, head, &selectorFlag)
	cmdMonitor.Init(kind, head, &selectorFlag)
	cmdPrintConfig.Init(kind, subPrint, &selectorFlag)
	cmdPrintConfigMtime.Init(kind, cmdPrintConfig.Command, &selectorFlag)
	cmdPrintStatus.Init(kind, subPrint, &selectorFlag)
	cmdRemove.Init(kind, head, &selectorFlag)
	cmdSet.Init(kind, head, &selectorFlag)
	cmdStatus.Init(kind, head, &selectorFlag)
	cmdUnset.Init(kind, head, &selectorFlag)
}
