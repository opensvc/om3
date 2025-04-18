package ox

import "github.com/opensvc/om3/core/commoncmd"

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
		commoncmd.NewCmdMonitor(),
	)
	cmdObject.AddCommand(
		cmdObjectConfig,
		cmdObjectEdit,
		cmdObjectSet,
		cmdObjectPrint,
		cmdObjectSSH,
		cmdObjectValidate,
		commoncmd.NewCmdClusterAbort(),
		commoncmd.NewCmdClusterFreeze(),
		commoncmd.NewCmdClusterLogs(),
		commoncmd.NewCmdClusterThaw(),
		commoncmd.NewCmdClusterStatus(),
		commoncmd.NewCmdClusterUnfreeze(),
		newCmdObjectCreate(kind),
		newCmdObjectEval(kind),
		newCmdObjectGet(kind),
		newCmdObjectLogs(kind),
		newCmdObjectList(kind),
		commoncmd.NewCmdObjectMonitor("", kind),
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
	)
	cmdObjectSSH.AddCommand(
		commoncmd.NewCmdClusterSSHTrust(),
	)
	cmdObjectValidate.AddCommand(
		newCmdObjectValidateConfig(kind),
	)
}
