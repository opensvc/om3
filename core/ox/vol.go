package ox

import "github.com/opensvc/om3/core/commoncmd"

func init() {
	kind := "vol"

	cmdObject := newCmdVol()
	cmdObjectCollector := newCmdObjectCollector(kind)
	cmdObjectCollectorTag := newCmdObjectCollectorTag(kind)
	cmdObjectConfig := newCmdObjectConfig(kind)
	cmdObjectEdit := newCmdObjectEdit(kind)
	cmdObjectInstance := newCmdObjectInstance(kind)
	cmdObjectInstanceDevice := newCmdObjectInstanceDevice(kind)
	cmdObjectSchedule := newCmdObjectSchedule(kind)
	cmdObjectSet := newCmdObjectSet(kind)
	cmdObjectPrint := newCmdObjectPrint(kind)
	cmdObjectPrintConfig := newCmdObjectPrintConfig(kind)
	cmdObjectPush := newCmdObjectPush(kind)
	cmdObjectResource := newCmdObjectResource(kind)
	cmdObjectResourceInfo := newCmdObjectResourceInfo(kind)
	cmdObjectSync := newCmdObjectSync(kind)
	cmdObjectValidate := newCmdObjectValidate(kind)

	root.AddCommand(
		cmdObject,
	)
	cmdObject.AddCommand(
		cmdObjectCollector,
		cmdObjectConfig,
		cmdObjectEdit,
		cmdObjectInstance,
		cmdObjectPrint,
		cmdObjectPush,
		cmdObjectResource,
		cmdObjectSet,
		cmdObjectSchedule,
		cmdObjectSync,
		cmdObjectValidate,
		newCmdObjectAbort(kind),
		newCmdObjectBoot(kind),
		newCmdObjectClear(kind),
		newCmdObjectCreate(kind),
		newCmdObjectDelete(kind),
		newCmdObjectEval(kind),
		newCmdObjectEnter(kind),
		newCmdObjectFreeze(kind),
		newCmdObjectGet(kind),
		newCmdObjectGiveback(kind),
		newCmdObjectLogs(kind),
		newCmdObjectList(kind),
		commoncmd.NewCmdObjectMonitor("", kind),
		newCmdObjectPurge(kind),
		newCmdObjectProvision(kind),
		newCmdObjectPRStart(kind),
		newCmdObjectPRStop(kind),
		newCmdObjectRestart(kind),
		newCmdObjectShutdown(kind),
		newCmdObjectStart(kind),
		newCmdObjectStartStandby(kind),
		newCmdObjectStop(kind),
		newCmdObjectSwitch(kind),
		newCmdObjectTakeover(kind),
		newCmdObjectThaw(kind),
		newCmdObjectUnfreeze(kind),
		newCmdObjectUnprovision(kind),
		newCmdObjectUnset(kind),
		newCmdObjectUpdate(kind),
		newCmdTUI(kind),
	)
	cmdObjectCollector.AddCommand(
		cmdObjectCollectorTag,
	)
	cmdObjectCollectorTag.AddCommand(
		newCmdObjectCollectorTagAttach(kind),
		newCmdObjectCollectorTagDetach(kind),
		newCmdObjectCollectorTagShow(kind),
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
	cmdObjectInstance.AddCommand(
		cmdObjectInstanceDevice,
		newCmdObjectInstanceFreeze(kind),
		newCmdObjectInstanceList(kind),
		newCmdObjectInstanceStatus(kind),
		newCmdObjectInstanceProvision(kind),
		newCmdObjectInstancePRStart(kind),
		newCmdObjectInstancePRStop(kind),
		newCmdObjectInstanceRun(kind),
		newCmdObjectInstanceStart(kind),
		newCmdObjectInstanceStop(kind),
		newCmdObjectInstanceUnfreeze(kind),
		newCmdObjectInstanceUnprovision(kind),
	)
	cmdObjectInstanceDevice.AddCommand(
		newCmdObjectInstanceDeviceList(kind),
	)
	cmdObjectResource.AddCommand(
		cmdObjectResourceInfo,
		newCmdObjectResourceList(kind),
	)
	cmdObjectResourceInfo.AddCommand(
		newCmdObjectResourceInfoList(kind),
		newCmdObjectResourceInfoPush(kind),
	)
	cmdObjectSchedule.AddCommand(
		newCmdObjectScheduleList(kind),
	)
	cmdObjectSet.AddCommand(
		newCmdObjectSetProvisioned(kind),
		newCmdObjectSetUnprovisioned(kind),
	)
	cmdObjectPrint.AddCommand(
		cmdObjectPrintConfig,
		newCmdObjectPrintResourceInfo(kind),
		newCmdObjectPrintSchedule(kind),
		newCmdObjectPrintStatus(kind),
	)
	cmdObjectPush.AddCommand(
		newCmdObjectPushResourceInfo(kind),
	)
	cmdObjectSync.AddCommand(
		newCmdObjectSyncIngest(kind),
		newCmdObjectSyncFull(kind),
		newCmdObjectSyncResync(kind),
		newCmdObjectSyncUpdate(kind),
	)
	cmdObjectValidate.AddCommand(
		newCmdObjectValidateConfig(kind),
	)
}
