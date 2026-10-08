package ox

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

func init() {
	cmdCtx := NewCmdContext()
	cmdCtxCluster := NewCmdContextCluster()
	cmdCtxUser := NewCmdContextUser()

	root.AddCommand(
		cmdCtx,
	)

	cmdCtx.AddGroup(
		commoncmd.NewGroupSubsystems(),
	)

	cmdCtxCluster.AddCommand(
		NewCmdContextClusterAdd(),
		NewCmdContextClusterChange(),
		NewCmdContextClusterRemove(),
	)

	cmdCtxUser.AddCommand(
		NewCmdContextUserAdd(),
		NewCmdContextUserChange(),
		NewCmdContextUserRemove(),
	)

	cmdCtx.AddCommand(
		cmdCtxCluster,
		cmdCtxUser,
		NewCmdContextLogin(),
		NewCmdContextLogout(),
		NewCmdContextList(),
		NewCmdContextShow(),
		NewCmdContextWhoAmI(),
		NewCmdContextEdit(),

		NewCmdContextAdd(),
		NewCmdContextChange(),
		NewCmdContextRemove(),
	)
}
