package cmd

func init() {
	kind := "ccfg"

	cmdObject := newCmdCcfg()
	cmdObjectEdit := newCmdObjectEdit(kind)
	cmdObjectSet := newCmdObjectSet(kind)
	cmdObjectPrint := newCmdObjectPrint(kind)
	cmdObjectPrintConfig := newCmdObjectPrintConfig(kind)
	cmdObjectValidate := newCmdObjectValidate(kind)

	root.AddCommand(
		cmdObject,
	)
	cmdObject.AddCommand(
		cmdObjectEdit,
		cmdObjectSet,
		cmdObjectPrint,
		cmdObjectValidate,
		newCmdObjectCreate(kind),
		newCmdObjectDoc(kind),
		newCmdObjectDelete(kind),
		newCmdObjectEval(kind),
		newCmdObjectGet(kind),
		newCmdObjectLogs(kind),
		newCmdObjectLs(kind),
		newCmdObjectMonitor(kind),
		newCmdObjectStatus(kind),
		newCmdObjectUnset(kind),
	)
	cmdObjectEdit.AddCommand(
		newCmdObjectEditConfig(kind),
	)
	cmdObjectPrint.AddCommand(
		cmdObjectPrintConfig,
		newCmdObjectPrintStatus(kind),
	)
	cmdObjectPrintConfig.AddCommand(
		newCmdObjectPrintConfigMtime(kind),
	)
	cmdObjectValidate.AddCommand(
		newCmdObjectValidateConfig(kind),
	)
}
