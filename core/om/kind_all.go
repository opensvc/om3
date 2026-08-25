package om

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/omcmd"
	"github.com/opensvc/om3/v3/util/hostname"
)

func init() {
	kind := ""
	cmdObject := commoncmd.NewCmdAll()
	cmdObjectCollector := commoncmd.NewCmdObjectCollector(kind)
	cmdObjectCollectorTag := newCmdObjectCollectorTag(kind)
	cmdObjectCompliance := commoncmd.NewCmdObjectCompliance(kind)
	cmdObjectComplianceAttach := newCmdObjectComplianceAttach(kind)
	cmdObjectComplianceDetach := newCmdObjectComplianceDetach(kind)
	cmdObjectComplianceShow := newCmdObjectComplianceShow(kind)
	cmdObjectComplianceList := newCmdObjectComplianceList(kind)
	cmdObjectConfig := commoncmd.NewCmdObjectConfig(kind)
	cmdObjectEdit := newCmdObjectEdit(kind)
	cmdObjectInstance := commoncmd.NewCmdObjectInstance(kind)
	cmdObjectInstancePG := commoncmd.NewCmdObjectInstancePG(kind)
	cmdObjectInstanceDevice := commoncmd.NewCmdObjectInstanceDevice(kind)
	cmdObjectPG := commoncmd.NewCmdObjectInstancePG(kind)
	cmdObjectPG.Hidden = true
	cmdObjectPrint := commoncmd.NewCmdObjectPrint(kind)
	cmdObjectPrintConfig := newCmdObjectPrintConfig(kind)
	cmdObjectPush := newCmdObjectPush(kind)
	cmdObjectSet := newCmdObjectSet(kind)
	cmdObjectSchedule := commoncmd.NewCmdObjectSchedule(kind)
	cmdObjectValidate := newCmdObjectValidate(kind)

	cmdObject.AddGroup(
		commoncmd.NewGroupOrchestrated(),
		commoncmd.NewGroupQuery(),
		commoncmd.NewGroupResources(),
		commoncmd.NewGroupSubsystems(),
	)
	root.AddCommand(
		cmdObject,
	)
	cmdObject.AddCommand(
		cmdObjectCollector,
		cmdObjectCompliance,
		cmdObjectConfig,
		newCmdDataStoreAdd(kind),
		newCmdDataStoreChange(kind),
		newCmdDataStoreDecode(kind),
		newCmdDataStoreKeys(kind),
		newCmdDataStoreInstall(kind),
		newCmdDataStoreRemove(kind),
		newCmdObjectKey(kind),
		newCmdObjectContainer(kind),
		newCmdObjectIP(kind),
		newCmdObjectFS(kind),
		newCmdObjectVolume(kind),
		newCmdObjectDisk(kind),
		newCmdObjectShare(kind),
		newCmdObjectApp(kind),
		newCmdObjectTask(kind),
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
		newCmdObjectAbort(kind),
		commoncmd.NewCmdObjectClear(kind),
		newCmdObjectCreate(kind),
		newCmdObjectDelete(kind),
		newCmdObjectDeploy(kind),
		newCmdObjectDisable(kind),
		newCmdObjectEnable(kind),
		newCmdObjectEnter(kind),
		newCmdObjectEval(kind),
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
	cmdObjectInstance.AddCommand(
		cmdObjectInstanceDevice,
		newCmdObjectInstanceBoot(kind),
		newCmdObjectInstanceDelete(kind),
		newCmdObjectInstanceFreeze(kind),
		newCmdObjectInstanceList(kind),
		newCmdObjectInstanceRun(kind),
		newCmdObjectInstanceStatus(kind),
		newCmdObjectInstanceProvision(kind),
		newCmdObjectInstancePRStart(kind),
		newCmdObjectInstancePRStop(kind),
		newCmdObjectInstanceRestart(kind),
		newCmdObjectInstanceShutdown(kind),
		newCmdObjectInstanceStart(kind),
		newCmdObjectInstanceStartStandby(kind),
		newCmdObjectInstanceStop(kind),
		newCmdObjectInstanceUnfreeze(kind),
		newCmdObjectInstanceUnprovision(kind),
		commoncmd.NewCmdObjectInstanceClear(kind, hostname.Hostname()),

		// sync
		newCmdObjectInstanceIngest(kind),
		newCmdObjectInstanceFull(kind),
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
	cmdObjectSchedule.AddCommand(
		newCmdObjectScheduleList(kind),
	)
	cmdObjectSet.AddCommand(
		//deprecated...
		newCmdObjectSetProvisioned(kind),
		newCmdObjectSetUnprovisioned(kind),
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
	cmdObjectValidate.AddCommand(
		newCmdObjectValidateConfig(kind),
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
	cmdObjectCompliance.AddCommand(
		cmdObjectComplianceAttach,
		cmdObjectComplianceDetach,
		cmdObjectComplianceShow,
		cmdObjectComplianceList,
		newCmdObjectComplianceEnv(kind),
		newCmdObjectComplianceAuto(kind),
		newCmdObjectComplianceCheck(kind),
		newCmdObjectComplianceFix(kind),
		newCmdObjectComplianceFixable(kind),
	)
	cmdObjectComplianceAttach.AddCommand(
		newCmdObjectComplianceAttachModuleset(kind),
		newCmdObjectComplianceAttachRuleset(kind),
	)
	cmdObjectComplianceDetach.AddCommand(
		newCmdObjectComplianceDetachModuleset(kind),
		newCmdObjectComplianceDetachRuleset(kind),
	)
	cmdObjectComplianceShow.AddCommand(
		newCmdObjectComplianceShowRuleset(kind),
		newCmdObjectComplianceShowModuleset(kind),
	)
	cmdObjectComplianceList.AddCommand(
		newCmdObjectComplianceListModules(kind),
		newCmdObjectComplianceListModuleset(kind),
		newCmdObjectComplianceListRuleset(kind),
	)
}
