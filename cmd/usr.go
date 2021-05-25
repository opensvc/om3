package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

var (
	subUsr = &cobra.Command{
		Use:   "usr",
		Short: "Manage users",
		Long: ` A user stores the grants and credentials of user of the agent API.

User objects are not necessary with OpenID authentication, as the
grants are embedded in the trusted bearer tokens.`,
	}
	subUsrEdit = &cobra.Command{
		Use:     "edit",
		Short:   "edit information about the object",
		Aliases: []string{"edi", "ed"},
	}
	subUsrPrint = &cobra.Command{
		Use:     "print",
		Short:   "print information about the object",
		Aliases: []string{"prin", "pri", "pr"},
	}
)

func init() {
	var (
		cmdCreate      commands.CmdObjectCreate
		cmdDelete      commands.CmdObjectDelete
		cmdEditConfig  commands.CmdObjectEditConfig
		cmdEval        commands.CmdObjectEval
		cmdGet         commands.CmdObjectGet
		cmdLs          commands.CmdObjectLs
		cmdMonitor     commands.CmdObjectMonitor
		cmdPrintConfig commands.CmdObjectPrintConfig
		cmdPrintStatus commands.CmdObjectPrintStatus
		cmdSet         commands.CmdObjectSet
		cmdStatus      commands.CmdObjectStatus
		cmdUnset       commands.CmdObjectUnset
	)

	kind := "usr"
	head := subUsr
	subEdit := subUsrEdit
	subPrint := subUsrPrint
	root := rootCmd

	root.AddCommand(head)
	head.AddCommand(subEdit)
	head.AddCommand(subPrint)

	cmdCreate.Init(kind, head, &selectorFlag)
	cmdDelete.Init(kind, head, &selectorFlag)
	cmdEditConfig.Init(kind, subEdit, &selectorFlag)
	cmdEval.Init(kind, head, &selectorFlag)
	cmdGet.Init(kind, head, &selectorFlag)
	cmdLs.Init(kind, head, &selectorFlag)
	cmdMonitor.Init(kind, head, &selectorFlag)
	cmdPrintConfig.Init(kind, subPrint, &selectorFlag)
	cmdPrintStatus.Init(kind, subPrint, &selectorFlag)
	cmdSet.Init(kind, head, &selectorFlag)
	cmdStatus.Init(kind, head, &selectorFlag)
	cmdUnset.Init(kind, head, &selectorFlag)
}
