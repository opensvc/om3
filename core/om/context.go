package om

import "github.com/opensvc/om3/core/commoncmd"

func init() {
	cmdCtx := commoncmd.NewCmdContext()
	root.AddCommand(
		cmdCtx,
	)

	cmdCtx.AddCommand(
		commoncmd.NewCmdDaemonLogin(),
		commoncmd.NewCmdDaemonLogout(),
	)
}
