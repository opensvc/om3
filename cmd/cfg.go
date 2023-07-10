package cmd

func init() {
	kind := "cfg"

	cmdObject := newCmdCfg()
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
		newCmdKeystoreAdd(kind),
		newCmdKeystoreChange(kind),
		newCmdKeystoreDecode(kind),
		newCmdKeystoreKeys(kind),
		newCmdKeystoreInstall(kind),
		newCmdKeystoreRemove(kind),
		newCmdObjectCreate(kind),
		newCmdObjectDoc(kind),
		newCmdObjectDelete(kind),
		newCmdObjectEval(kind),
		newCmdObjectGet(kind),
		newCmdObjectLs(kind),
		newCmdObjectLogs(kind),
		newCmdObjectMonitor(kind),
		newCmdObjectPurge(kind),
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
