package om

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/omcmd"
	"github.com/opensvc/om3/v3/util/hostname"
)

func init() {
	kind := "svc"

	cmdObject := newCmdSVC()
	cmdObjectCollector := commoncmd.NewCmdObjectCollector(kind)
	cmdObjectCollectorTag := newCmdObjectCollectorTag(kind)
	cmdObjectCompliance := commoncmd.NewCmdObjectCompliance(kind)
	cmdObjectComplianceAttach := newCmdObjectComplianceAttach(kind)
	cmdObjectComplianceDetach := newCmdObjectComplianceDetach(kind)
	cmdObjectComplianceShow := newCmdObjectComplianceShow(kind)
	cmdObjectComplianceList := newCmdObjectComplianceList(kind)
	cmdObjectConfig := commoncmd.NewCmdObjectConfig(kind)
	cmdObjectContainer := commoncmd.NewCmdObjectContainer(kind)
	cmdObjectIP := commoncmd.NewCmdObjectIP(kind)
	cmdObjectFS := commoncmd.NewCmdObjectFS(kind)
	cmdObjectVolume := commoncmd.NewCmdObjectVolume(kind)
	cmdObjectDisk := commoncmd.NewCmdObjectDisk(kind)
	cmdObjectShare := commoncmd.NewCmdObjectShare(kind)
	cmdObjectApp := commoncmd.NewCmdObjectApp(kind)
	cmdObjectTask := commoncmd.NewCmdObjectTask(kind)
	cmdObjectEdit := newCmdObjectEdit(kind)
	cmdObjectInstance := commoncmd.NewCmdObjectInstance(kind)
	cmdObjectInstanceDevice := commoncmd.NewCmdObjectInstanceDevice(kind)
	cmdObjectInstancePG := commoncmd.NewCmdObjectInstancePG(kind)
	cmdObjectInstanceSync := commoncmd.NewCmdObjectInstanceSync(kind)
	cmdObjectPG := commoncmd.NewCmdObjectInstancePG(kind)
	cmdObjectPG.Hidden = true
	cmdObjectPrint := newCmdObjectPrint(kind)
	cmdObjectPrintConfig := newCmdObjectPrintConfig(kind)
	cmdObjectPush := newCmdObjectPush(kind)
	cmdObjectResource := commoncmd.NewCmdObjectResource(kind)
	cmdObjectResourceInfo := commoncmd.NewCmdObjectResourceInfo(kind)
	cmdObjectSchedule := commoncmd.NewCmdObjectSchedule(kind)
	cmdObjectSet := newCmdObjectSet(kind)
	cmdObjectSync := commoncmd.NewCmdObjectSync(kind)
	cmdObjectValidate := newCmdObjectValidate(kind)

	root.AddCommand(
		cmdObject,
	)
	cmdObject.AddGroup(
		commoncmd.NewGroupOrchestratedActions(),
		commoncmd.NewGroupQuery(),
		commoncmd.NewGroupResources(),
		commoncmd.NewGroupSubsystems(),
	)
	cmdObject.AddCommand(
		cmdObjectCollector,
		cmdObjectCompliance,
		cmdObjectConfig,
		cmdObjectContainer,
		cmdObjectIP,
		cmdObjectFS,
		cmdObjectVolume,
		cmdObjectDisk,
		cmdObjectShare,
		cmdObjectApp,
		cmdObjectTask,
		cmdObjectEdit,
		cmdObjectInstance,
		cmdObjectPG,
		cmdObjectPrint,
		cmdObjectPush,
		cmdObjectResource,
		cmdObjectSchedule,
		cmdObjectSet,
		cmdObjectSync,
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
	cmdObjectContainer.AddCommand(
		newCmdObjectContainerEnter(kind),
		newCmdObjectContainerLogs(kind),
		newCmdObjectContainerList(kind),
		newCmdObjectContainerStart(kind),
		newCmdObjectContainerStop(kind),
		newCmdObjectContainerRestart(kind),
		newCmdObjectContainerProvision(kind),
		newCmdObjectContainerUnprovision(kind),
	)
	cmdObjectIP.AddCommand(
		newCmdObjectIPStart(kind),
		newCmdObjectIPStop(kind),
		newCmdObjectIPRestart(kind),
		newCmdObjectIPProvision(kind),
		newCmdObjectIPUnprovision(kind),
	)
	cmdObjectFS.AddCommand(
		newCmdObjectFSStart(kind),
		newCmdObjectFSStop(kind),
		newCmdObjectFSRestart(kind),
		newCmdObjectFSProvision(kind),
		newCmdObjectFSUnprovision(kind),
	)
	cmdObjectVolume.AddCommand(
		newCmdObjectVolumeStart(kind),
		newCmdObjectVolumeStop(kind),
		newCmdObjectVolumeRestart(kind),
		newCmdObjectVolumeProvision(kind),
		newCmdObjectVolumeUnprovision(kind),
	)
	cmdObjectDisk.AddCommand(
		newCmdObjectDiskStart(kind),
		newCmdObjectDiskStop(kind),
		newCmdObjectDiskRestart(kind),
		newCmdObjectDiskProvision(kind),
		newCmdObjectDiskUnprovision(kind),
	)
	cmdObjectShare.AddCommand(
		newCmdObjectShareStart(kind),
		newCmdObjectShareStop(kind),
		newCmdObjectShareRestart(kind),
		newCmdObjectShareProvision(kind),
		newCmdObjectShareUnprovision(kind),
	)
	cmdObjectApp.AddCommand(
		newCmdObjectAppStart(kind),
		newCmdObjectAppStop(kind),
		newCmdObjectAppRestart(kind),
	)
	cmdObjectTask.AddCommand(
		newCmdObjectTaskList(kind),
		newCmdObjectTaskRun(kind),
	)
	cmdObjectEdit.AddCommand(
		newCmdObjectEditConfig(kind),
	)
	cmdObjectPG.AddCommand(
		newCmdObjectInstancePGUpdate(kind),
	)
	cmdObjectResource.AddCommand(
		newCmdObjectResourceList(kind),
		cmdObjectResourceInfo,
	)
	cmdObjectInstance.AddCommand(
		cmdObjectInstanceDevice,
		cmdObjectInstancePG,
		cmdObjectInstanceSync,
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
	)
	cmdObjectInstanceDevice.AddCommand(
		newCmdObjectInstanceDeviceList(kind),
	)
	cmdObjectInstancePG.AddCommand(
		newCmdObjectInstancePGUpdate(kind),
	)

	cmdObjectResourceInfo.AddCommand(
		newCmdObjectResourceInfoList(kind),
		newCmdObjectResourceInfoPush(kind),
	)
	cmdObjectInstanceSync.AddCommand(
		newCmdObjectInstanceSyncFull(kind),
		newCmdObjectInstanceSyncIngest(kind),
		newCmdObjectInstanceSyncResync(kind),
		newCmdObjectInstanceSyncSplit(kind),
		newCmdObjectInstanceSyncUpdate(kind),
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
		newCmdObjectSyncFull(kind),
		newCmdObjectSyncIngest(kind),
		newCmdObjectSyncResync(kind),
		newCmdObjectSyncSplit(kind),
		newCmdObjectSyncUpdate(kind),
		newCmdObjectSyncList(kind),
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
