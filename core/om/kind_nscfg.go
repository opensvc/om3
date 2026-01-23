package om

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
	cmdObjectInstance := commoncmd.NewCmdObjectInstance(kind)

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
		cmdObjectInstance,
		newCmdObjectCreate(kind),
		newCmdObjectDelete(kind),
		newCmdObjectAbort(kind),
		newCmdObjectList(kind),
		commoncmd.NewCmdObjectMonitor("", kind),
	)

	cmdObjectInstance.AddCommand(
		newCmdObjectInstanceClear(kind),
		newCmdObjectInstanceList(kind),
		newCmdObjectInstanceDelete(kind),
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
}
