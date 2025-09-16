package om

import (
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/util/hostname"
)

func init() {
	kind := ""
	cmdObject := newCmdAll()
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
	cmdObjectInstanceDevice := commoncmd.NewCmdObjectInstanceDevice(kind)
	cmdObjectInstanceResource := commoncmd.NewCmdObjectInstanceResource(kind)
	cmdObjectInstanceResourceInfo := commoncmd.NewCmdObjectInstanceResourceInfo(kind)
	cmdObjectSchedule := commoncmd.NewCmdObjectSchedule(kind)
	cmdObjectSet := newCmdObjectSet(kind)
	cmdObjectPrint := newCmdObjectPrint(kind)
	cmdObjectPrintConfig := newCmdObjectPrintConfig(kind)
	cmdObjectResource := commoncmd.NewCmdObjectResource(kind)
	cmdObjectPush := newCmdObjectPush(kind)
	cmdObjectSync := commoncmd.NewCmdObjectSync(kind)
	cmdObjectInstanceSync := commoncmd.NewCmdObjectInstanceSync(kind)
	cmdObjectValidate := newCmdObjectValidate(kind)

	cmdObject.AddGroup(
		commoncmd.NewGroupOrchestratedActions(),
		commoncmd.NewGroupQuery(),
		commoncmd.NewGroupSubsystems(),
	)
	root.AddCommand(
		cmdObject,
	)
	cmdObject.AddCommand(
		cmdObjectCollector,
		cmdObjectCompliance,
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
		commoncmd.NewCmdObjectClear(kind),
		newCmdObjectCreate(kind),
		newCmdObjectDelete(kind),
		newCmdObjectDeploy(kind),
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
		cmdObjectInstanceSync,
		cmdObjectInstanceResource,
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
	)
	cmdObjectInstanceDevice.AddCommand(
		newCmdObjectInstanceDeviceList(kind),
	)
	cmdObjectInstanceResource.AddCommand(
		cmdObjectInstanceResourceInfo,
	)
	cmdObjectInstanceResourceInfo.AddCommand(
		newCmdObjectInstanceResourceInfoList(kind),
		newCmdObjectInstanceResourceInfoPush(kind),
	)
	cmdObjectInstanceSync.AddCommand(
		newCmdObjectInstanceSyncIngest(kind),
		newCmdObjectInstanceSyncFull(kind),
		newCmdObjectInstanceSyncResync(kind),
		newCmdObjectInstanceSyncUpdate(kind),
	)
	cmdObjectResource.AddCommand(
		newCmdObjectResourceList(kind),
	)
	cmdObjectConfig.AddCommand(
		commoncmd.NewCmdObjectConfigDoc(kind),
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
	cmdObjectSync.AddCommand(
		newCmdObjectInstanceSyncFull(kind),
		newCmdObjectInstanceSyncIngest(kind),
		newCmdObjectInstanceSyncResync(kind),
		newCmdObjectInstanceSyncUpdate(kind),
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
