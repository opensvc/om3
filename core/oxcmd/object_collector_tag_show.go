package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectCollectorTagShow struct {
		OptsGlobal
		Verbose bool
	}
)

// Run shows the tags attached to the selected objects.
func (t *CmdObjectCollectorTagShow) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return commoncmd.ObjectsAPIDo(mergedSelector, t.OptsGlobal.OptsGlobal, commoncmd.ObjectCollectorTagShow(t.Verbose))
}
