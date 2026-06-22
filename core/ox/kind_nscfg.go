package ox

import (
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/omcmd"
)

func init() {
	kind := "nscfg"

	cmdObject := &cobra.Command{
		Use:   "nscfg",
		Short: "manage namespace configurations",
	}
	cmdObjectConfig := commoncmd.NewCmdObjectConfig(kind)
	cmdObjectEdit := newCmdObjectEdit(kind)
	cmdObjectInstance := commoncmd.NewCmdObjectInstance(kind)
	cmdObjectInstancePG := commoncmd.NewCmdObjectInstancePG(kind)
	cmdObjectPG := commoncmd.NewCmdObjectInstancePG(kind)
	cmdObjectPG.Hidden = true

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
		cmdObjectInstance,
		cmdObjectPG,
		newCmdObjectCreate(kind),
		newCmdObjectDelete(kind),
		newCmdObjectAbort(kind),
		newCmdObjectList(kind),
		commoncmd.NewCmdObjectMonitor("", kind),
	)
	cmdObjectEdit.AddCommand(
		newCmdObjectEditConfig(kind),
	)
	cmdObjectInstance.AddCommand(
		cmdObjectInstancePG,
		newCmdObjectInstanceBoot(kind),
		commoncmd.NewCmdObjectClear(kind),
		newCmdObjectInstanceList(kind),
		newCmdObjectInstanceDelete(kind),
	)

	cmdObjectConfig.AddCommand(
		omcmd.NewCmdObjectConfigDoc(kind),
		newCmdObjectConfigEdit(kind),
		newCmdObjectConfigEval(kind),
		newCmdObjectConfigGet(kind),
		newCmdObjectConfigShow(kind),
		newCmdObjectConfigUpdate(kind),
		newCmdObjectConfigValidate(kind),
	)

	cmdObjectInstancePG.AddCommand(
		newCmdObjectInstancePGUpdate(kind),
	)
	cmdObjectPG.AddCommand(
		newCmdObjectInstancePGUpdate(kind),
	)
}
