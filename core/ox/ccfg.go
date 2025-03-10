package ox

func init() {
	kind := "ccfg"

	cmdObject := newCmdCcfg()
	cmdObjectConfig := newCmdObjectConfig(kind)
	cmdObjectEdit := newCmdObjectEdit(kind)
	cmdObjectSet := newCmdObjectSet(kind)
	cmdObjectPrint := newCmdObjectPrint(kind)
	cmdObjectPrintConfig := newCmdObjectPrintConfig(kind)
	cmdObjectSSH := newCmdObjectSSH(kind)
	cmdObjectValidate := newCmdObjectValidate(kind)

	root.AddCommand(
		cmdObject,
	)
	cmdObject.AddCommand(
		cmdObjectConfig,
		cmdObjectEdit,
		cmdObjectSet,
		cmdObjectPrint,
		cmdObjectSSH,
		cmdObjectValidate,
		newCmdClusterAbort(),
		newCmdClusterFreeze(),
		newCmdClusterLogs(),
		newCmdClusterThaw(),
		newCmdClusterUnfreeze(),
		newCmdObjectCreate(kind),
		newCmdObjectEval(kind),
		newCmdObjectGet(kind),
		newCmdObjectLogs(kind),
		newCmdObjectLs(kind),
		newCmdObjectMonitor(kind),
		newCmdObjectStatus(kind),
		newCmdObjectUnset(kind),
		newCmdObjectUpdate(kind),
		newCmdTUI(kind),
	)
	cmdObjectConfig.AddCommand(
		newCmdObjectConfigEdit(kind),
		newCmdObjectConfigEval(kind),
		newCmdObjectConfigGet(kind),
		newCmdObjectConfigShow(kind),
		newCmdObjectConfigUpdate(kind),
		newCmdObjectConfigValidate(kind),
	)
	cmdObjectEdit.AddCommand(
		newCmdObjectEditConfig(kind),
	)
	cmdObjectPrint.AddCommand(
		cmdObjectPrintConfig,
		newCmdObjectPrintStatus(kind),
	)
	cmdObjectSSH.AddCommand(
		newCmdClusterSSHTrust(),
	)
	cmdObjectValidate.AddCommand(
		newCmdObjectValidateConfig(kind),
	)
}
