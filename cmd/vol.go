package cmd

func init() {
	kind := "vol"

	cmdObject := newCmdVol()
	cmdObjectEdit := newCmdObjectEdit(kind)
	cmdObjectSet := newCmdObjectSet(kind)
	cmdObjectPrint := newCmdObjectPrint(kind)
	cmdObjectPrintConfig := newCmdObjectPrintConfig(kind)
	cmdObjectPush := newCmdObjectPush(kind)
	cmdObjectSync := newCmdObjectSync(kind)
	cmdObjectValidate := newCmdObjectValidate(kind)

	root.AddCommand(
		cmdObject,
	)
	cmdObject.AddCommand(
		cmdObjectEdit,
		cmdObjectPrint,
		cmdObjectPush,
		cmdObjectSet,
		cmdObjectSync,
		newCmdObjectAbort(kind),
		newCmdObjectClear(kind),
		newCmdObjectCreate(kind),
		newCmdObjectDelete(kind),
		newCmdObjectDoc(kind),
		newCmdObjectEval(kind),
		newCmdObjectEnter(kind),
		newCmdObjectFreeze(kind),
		newCmdObjectGet(kind),
		newCmdObjectGiveback(kind),
		newCmdObjectLogs(kind),
		newCmdObjectLs(kind),
		newCmdObjectMonitor(kind),
		newCmdObjectPurge(kind),
		newCmdObjectProvision(kind),
		newCmdObjectPRStart(kind),
		newCmdObjectPRStop(kind),
		newCmdObjectRestart(kind),
		newCmdObjectRun(kind),
		newCmdObjectShutdown(kind),
		newCmdObjectStart(kind),
		newCmdObjectStatus(kind),
		newCmdObjectStop(kind),
		newCmdObjectSwitch(kind),
		newCmdObjectThaw(kind),
		newCmdObjectUnfreeze(kind),
		newCmdObjectUnprovision(kind),
		newCmdObjectUnset(kind),
	)
	cmdObjectEdit.AddCommand(
		newCmdObjectEditConfig(kind),
	)
	cmdObjectSet.AddCommand(
		newCmdObjectSetProvisioned(kind),
		newCmdObjectSetUnprovisioned(kind),
	)
	cmdObjectPrint.AddCommand(
		cmdObjectPrintConfig,
		newCmdObjectPrintDevices(kind),
		newCmdObjectPrintSchedule(kind),
		newCmdObjectPrintStatus(kind),
	)
	cmdObjectPrintConfig.AddCommand(
		newCmdObjectPrintConfigMtime(kind),
	)
	cmdObjectPush.AddCommand(
		newCmdObjectPushResInfo(kind),
	)
	cmdObjectSync.AddCommand(
		newCmdObjectSyncResync(kind),
	)
	cmdObjectValidate.AddCommand(
		newCmdObjectValidateConfig(kind),
	)
}
