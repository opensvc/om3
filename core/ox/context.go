package ox

import "github.com/opensvc/om3/core/commoncmd"

func init() {
	cmdCtx := commoncmd.NewCmdContext()
	cmdCtxCluster := commoncmd.NewCmdContextCluster()
	cmdCtxUser := commoncmd.NewCmdContextUser()

	root.AddCommand(
		cmdCtx,
	)

	cmdCtxCluster.AddCommand(
		commoncmd.NewCmdContextClusterAdd(),
		commoncmd.NewCmdContextClusterChange(),
		commoncmd.NewCmdContextClusterRemove(),
	)

	cmdCtxUser.AddCommand(
		commoncmd.NewCmdContextUserAdd(),
		commoncmd.NewCmdContextUserChange(),
		commoncmd.NewCmdContextUserRemove(),
	)

	cmdCtx.AddCommand(
		cmdCtxCluster,
		cmdCtxUser,
		commoncmd.NewCmdContextLogin(),
		commoncmd.NewCmdContextLogout(),
		commoncmd.NewCmdContextList(),
		commoncmd.NewCmdContextShow(),
		commoncmd.NewCmdContextEdit(),

		commoncmd.NewCmdContextAdd(),
		commoncmd.NewCmdContextChange(),
		commoncmd.NewCmdContextRemove(),
	)
}
