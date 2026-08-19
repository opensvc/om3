package om

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/omcmd"
	"github.com/opensvc/om3/v3/util/hostname"
)

func init() {
	kind := "vol"

	cmdObject := commoncmd.NewCmdVol()
	cmdObjectCollector := commoncmd.NewCmdObjectCollector(kind)
	cmdObjectCollectorTag := newCmdObjectCollectorTag(kind)
	cmdObjectEdit := newCmdObjectEdit(kind)
	cmdObjectConfig := commoncmd.NewCmdObjectConfig(kind)
	cmdObjectInstance := commoncmd.NewCmdObjectInstance(kind)
	cmdObjectInstanceDevice := commoncmd.NewCmdObjectInstanceDevice(kind)
	cmdObjectInstancePG := commoncmd.NewCmdObjectInstancePG(kind)
	cmdObjectPG := commoncmd.NewCmdObjectInstancePG(kind)
	cmdObjectPG.Hidden = true
	cmdObjectPrint := commoncmd.NewCmdObjectPrint(kind)
	cmdObjectPrintConfig := newCmdObjectPrintConfig(kind)
	cmdObjectPush := newCmdObjectPush(kind)
	cmdObjectSchedule := commoncmd.NewCmdObjectSchedule(kind)
	cmdObjectSet := newCmdObjectSet(kind)
	cmdObjectValidate := newCmdObjectValidate(kind)

	root.AddCommand(
		cmdObject,
	)
	cmdObject.AddGroup(
		commoncmd.NewGroupOrchestrated(),
		commoncmd.NewGroupQuery(),
		commoncmd.NewGroupReplication(),
		commoncmd.NewGroupResources(),
		commoncmd.NewGroupSubsystems(),
	)
	cmdObject.AddCommand(
		cmdObjectCollector,
		cmdObjectConfig,
		newCmdObjectFS(kind),
		newCmdObjectDisk(kind),
		cmdObjectEdit,
		cmdObjectInstance,
		cmdObjectPG,
		cmdObjectPrint,
		cmdObjectPush,
		newCmdObjectResource(kind),
		cmdObjectSet,
		cmdObjectSchedule,
		newCmdObjectSync(kind),
		cmdObjectValidate,
		newCmdObjectVolume(kind),
		newCmdObjectAbort(kind),
		commoncmd.NewCmdObjectClear(kind),
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
		newCmdObjectRun(kind),
		newCmdObjectShutdown(kind),
		newCmdObjectStart(kind),
		newCmdObjectStartStandby(kind),
		newCmdObjectStatus(kind),
		newCmdObjectStop(kind),
		newCmdObjectSwitch(kind),
		newCmdObjectTakeover(kind),
		newCmdObjectThaw(kind),
		newCmdObjectUnfreeze(kind),
		newCmdObjectUnprovision(kind),
		newCmdObjectUnset(kind),
	)
	cmdObjectCollector.AddCommand(
		cmdObjectCollectorTag,
	)
	cmdObjectCollectorTag.AddCommand(
		newCmdObjectCollectorTagAttach(kind),
		newCmdObjectCollectorTagCreate(kind),
		newCmdObjectCollectorTagDetach(kind),
		newCmdObjectCollectorTagList(kind),
		newCmdObjectCollectorTagShow(kind),
	)
	cmdObjectConfig.AddCommand(
		omcmd.NewCmdObjectConfigDoc(kind),
		newCmdObjectConfigEdit(kind),
		newCmdObjectConfigEval(kind),
		newCmdObjectConfigGet(kind),
		newCmdObjectConfigMtime(kind),
		newCmdObjectConfigShow(kind),
		newCmdObjectConfigUpdate(kind),
		newCmdObjectConfigValidate(kind),
	)
	cmdObjectEdit.AddCommand(
		newCmdObjectEditConfig(kind),
	)
	cmdObjectInstance.AddCommand(
		cmdObjectInstanceDevice,
		cmdObjectInstancePG,
		newCmdObjectInstanceBoot(kind),
		newCmdObjectInstanceDelete(kind),
		newCmdObjectInstanceFreeze(kind),
		newCmdObjectInstanceList(kind),
		newCmdObjectInstancePRStart(kind),
		newCmdObjectInstancePRStop(kind),
		newCmdObjectInstanceProvision(kind),
		newCmdObjectInstanceRestart(kind),
		newCmdObjectInstanceRun(kind),
		newCmdObjectInstanceShutdown(kind),
		newCmdObjectInstanceStart(kind),
		newCmdObjectInstanceStartStandby(kind),
		newCmdObjectInstanceStatus(kind),
		newCmdObjectInstanceStop(kind),
		newCmdObjectInstanceUnfreeze(kind),
		newCmdObjectInstanceUnprovision(kind),
		commoncmd.NewCmdObjectInstanceClear(kind, hostname.Hostname()),

		// sync
		newCmdObjectInstanceFull(kind),
		newCmdObjectInstanceIngest(kind),
		newCmdObjectInstanceResync(kind),
		newCmdObjectInstanceSplit(kind),
		newCmdObjectInstanceUpdate(kind),
	)
	cmdObjectInstanceDevice.AddCommand(
		newCmdObjectInstanceDeviceList(kind),
	)
	cmdObjectInstancePG.AddCommand(
		newCmdObjectInstancePGUpdate(kind),
	)
	cmdObjectPG.AddCommand(
		newCmdObjectInstancePGUpdate(kind),
	)
	cmdObjectPrint.AddCommand(
		cmdObjectPrintConfig,
		newCmdObjectPrintResourceInfo(kind),
		newCmdObjectPrintSchedule(kind),
		newCmdObjectPrintStatus(kind),
	)
	cmdObjectPrintConfig.AddCommand(
		newCmdObjectConfigMtime(kind),
	)
	cmdObjectPush.AddCommand(
		newCmdObjectPushResourceInfo(kind),
	)
	cmdObjectSchedule.AddCommand(
		newCmdObjectScheduleList(kind),
	)
	cmdObjectSet.AddCommand(
		//deprecated...
		newCmdObjectSetProvisioned(kind),
		newCmdObjectSetUnprovisioned(kind),
	)
	cmdObjectValidate.AddCommand(
		newCmdObjectValidateConfig(kind),
	)
}
