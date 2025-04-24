package om

import "github.com/opensvc/om3/core/commoncmd"

func init() {
	root.AddGroup(
		commoncmd.NewGroupQuery(),
	)
	root.AddCommand(
		commoncmd.NewCmdMonitor(),
	)
}
