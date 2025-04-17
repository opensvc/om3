package ox

import "github.com/opensvc/om3/core/commoncmd"

func init() {
	root.AddCommand(
		commoncmd.NewCmdMonitor(),
	)
}
