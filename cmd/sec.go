package cmd

func init() {
	kind := "sec"

	cmdObject := newCmdSec()
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
		newCmdSecGenCert(kind),
		newCmdSecPKCS(kind),
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
