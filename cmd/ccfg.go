package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

func makeSubCcfg() *cobra.Command {
	return &cobra.Command{
		Use:   "ccfg",
		Short: "Manage the cluster shared configuration",
		Long: ` The cluster nodes merge their private configuration
over the cluster shared configuration.

The shared configuration is hosted in a ccfg-kind object, and is
replicated using the same rules as other kinds of object (last write is
eventually replicated).
`,
	}
}

func init() {
	var (
		cmdCreate           commands.CmdObjectCreate
		cmdDoc              commands.CmdObjectDoc
		cmdDelete           commands.CmdObjectDelete
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
		cmdValidateConfig   commands.CmdObjectValidateConfig
	)

	kind := "ccfg"
	head := makeSubCcfg()
	root.AddCommand(head)

	cmdCreate.Init(kind, head, &selectorFlag)
	cmdDoc.Init(kind, head, &selectorFlag)
	cmdDelete.Init(kind, head, &selectorFlag)
	cmdEval.Init(kind, head, &selectorFlag)
	cmdGet.Init(kind, head, &selectorFlag)
	cmdLs.Init(kind, head, &selectorFlag)
	cmdMonitor.Init(kind, head, &selectorFlag)
	cmdSet.Init(kind, head, &selectorFlag)
	cmdStatus.Init(kind, head, &selectorFlag)
	cmdUnset.Init(kind, head, &selectorFlag)

	if sub := makeSubEdit(); sub != nil {
		head.AddCommand(sub)
		cmdEditConfig.Init(kind, sub, &selectorFlag)
	}

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
