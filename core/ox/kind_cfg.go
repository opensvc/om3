package ox

import "github.com/opensvc/om3/v3/core/commoncmd"

func init() {
	kind := "cfg"

	cmdObject := commoncmd.NewCmdCfg()
	cmdObjectConfig := commoncmd.NewCmdObjectConfig(kind)
	cmdObjectEdit := newCmdObjectEdit(kind)
	cmdObjectInstance := commoncmd.NewCmdObjectInstance(kind)
	cmdObjectSet := newCmdObjectSet(kind)
	cmdObjectPrint := commoncmd.NewCmdObjectPrint(kind)
	cmdObjectPrintConfig := newCmdObjectPrintConfig(kind)
	cmdObjectValidate := newCmdObjectValidate(kind)

	root.AddCommand(
		cmdObject,
	)
	cmdObject.AddGroup(
		commoncmd.NewGroupOrchestrated(),
		commoncmd.NewGroupQuery(),
		commoncmd.NewGroupSubsystems(),
	)
	cmdObject.AddCommand(
		cmdObjectConfig,
		cmdObjectEdit,
		cmdObjectInstance,
		cmdObjectPrint,
		cmdObjectSet,
		cmdObjectValidate,
		newCmdDataStoreAdd(kind),
		newCmdDataStoreChange(kind),
		newCmdDataStoreDecode(kind),
		newCmdDataStoreKeys(kind),
		newCmdDataStoreInstall(kind),
		newCmdDataStoreRemove(kind),
		newCmdObjectKey(kind),
		newCmdObjectCreate(kind),
		newCmdObjectDelete(kind),
		newCmdObjectEval(kind),
		newCmdObjectGet(kind),
		newCmdObjectLogs(kind),
		newCmdObjectList(kind),
		commoncmd.NewCmdObjectMonitor("", kind),
		newCmdObjectPurge(kind),
		newCmdObjectUnset(kind),
		newCmdObjectUpdate(kind),
		newCmdTUI(kind),
	)
	cmdObjectConfig.AddCommand(
		commoncmd.NewCmdObjectConfigDoc(kind),
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
		newCmdObjectInstanceList(kind),
		newCmdObjectInstanceDelete(kind),
	)
	cmdObjectPrint.AddCommand(
		cmdObjectPrintConfig,
	)
	cmdObjectValidate.AddCommand(
		newCmdObjectValidateConfig(kind),
	)
}
