package om

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/omcmd"
)

func init() {
	kind := "ccfg"

	cmdObject := newCmdCcfg()
	cmdObjectConfig := commoncmd.NewCmdObjectConfig(kind)
	cmdObjectEdit := newCmdObjectEdit(kind)
	cmdObjectSet := newCmdObjectSet(kind)
	cmdObjectSSH := commoncmd.NewCmdObjectSSH(kind)
	cmdObjectPrint := newCmdObjectPrint(kind)
	cmdObjectPrintConfig := newCmdObjectPrintConfig(kind)
	cmdObjectValidate := newCmdObjectValidate(kind)

	root.AddCommand(
		cmdObject,
	)
	cmdObject.AddGroup(
		commoncmd.NewGroupOrchestratedActions(),
		commoncmd.NewGroupQuery(),
		commoncmd.NewGroupSubsystems(),
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
		newCmdObjectList(kind),
		commoncmd.NewCmdObjectMonitor("", kind),
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
