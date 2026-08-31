package om

import "github.com/opensvc/om3/v3/core/commoncmd"

func init() {
	root.AddCommand(
		commoncmd.NewCmdMonitor(),
	)
}
