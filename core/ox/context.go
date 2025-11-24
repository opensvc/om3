package ox

import (
	"github.com/opensvc/om3/core/commoncmd"
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
		NewCmdContextEdit(),

		NewCmdContextAdd(),
		NewCmdContextChange(),
		NewCmdContextRemove(),
	)
}
