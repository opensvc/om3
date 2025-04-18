package om

import "github.com/opensvc/om3/core/commoncmd"

func init() {
	kind := "ccfg"

	cmdObject := newCmdCcfg()
	cmdObjectConfig := newCmdObjectConfig(kind)
	cmdObjectEdit := newCmdObjectEdit(kind)
	cmdObjectSet := newCmdObjectSet(kind)
	cmdObjectSSH := newCmdObjectSSH(kind)
	cmdObjectPrint := newCmdObjectPrint(kind)
	cmdObjectPrintConfig := newCmdObjectPrintConfig(kind)
	cmdObjectValidate := newCmdObjectValidate(kind)

	root.AddCommand(
		cmdObject,
	)
	cmdObject.AddCommand(
		cmdObjectConfig,
		cmdObjectEdit,
		cmdObjectSet,
		cmdObjectSSH,
		cmdObjectPrint,
		cmdObjectValidate,
		newCmdClusterJoin(),
		newCmdClusterLeave(),
		commoncmd.NewCmdClusterAbort(),
		commoncmd.NewCmdClusterFreeze(),
		commoncmd.NewCmdClusterLogs(),
		commoncmd.NewCmdClusterStatus(),
		commoncmd.NewCmdClusterThaw(),
		commoncmd.NewCmdClusterUnfreeze(),
		newCmdObjectCreate(kind),
		newCmdObjectEval(kind),
		newCmdObjectGet(kind),
		newCmdObjectLogs(kind),
		newCmdObjectList(kind),
		commoncmd.NewCmdObjectMonitor("", kind),
		newCmdObjectUnset(kind),
	)
	cmdObjectConfig.AddCommand(
		newCmdObjectConfigDoc(kind),
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
	cmdObjectPrint.AddCommand(
		cmdObjectPrintConfig,
	)
	cmdObjectPrintConfig.AddCommand(
		newCmdObjectConfigMtime(kind),
	)
	cmdObjectSSH.AddCommand(
		commoncmd.NewCmdClusterSSHTrust(),
	)
	cmdObjectValidate.AddCommand(
		newCmdObjectValidateConfig(kind),
	)
}
