package cmd

func init() {
	kind := "cfg"

	cmdObject := newCmdCfg()
	cmdObjectEdit := newCmdObjectEdit(kind)
	cmdObjectInstance := newCmdObjectInstance(kind)
	cmdObjectSet := newCmdObjectSet(kind)
	cmdObjectPrint := newCmdObjectPrint(kind)
	cmdObjectPrintConfig := newCmdObjectPrintConfig(kind)
	cmdObjectValidate := newCmdObjectValidate(kind)

	root.AddCommand(
		cmdObject,
	)
	cmdObject.AddCommand(
		cmdObjectEdit,
		cmdObjectInstance,
		cmdObjectPrint,
		cmdObjectSet,
		cmdObjectValidate,
		newCmdKeystoreAdd(kind),
		newCmdKeystoreChange(kind),
		newCmdKeystoreDecode(kind),
		newCmdKeystoreKeys(kind),
		newCmdKeystoreInstall(kind),
		newCmdKeystoreRemove(kind),
		newCmdObjectCreate(kind),
		newCmdObjectDelete(kind),
		newCmdObjectDoc(kind),
		newCmdObjectEval(kind),
		newCmdObjectGet(kind),
		newCmdObjectLogs(kind),
		newCmdObjectLs(kind),
		newCmdObjectMonitor(kind),
		newCmdObjectPurge(kind),
		newCmdObjectStatus(kind),
		newCmdObjectUnset(kind),
	)
	cmdObjectEdit.AddCommand(
		newCmdObjectEditConfig(kind),
	)
	cmdObjectInstance.AddCommand(
		newCmdObjectInstanceLs(kind),
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
