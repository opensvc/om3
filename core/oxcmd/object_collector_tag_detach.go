package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectCollectorTagDetach struct {
		OptsGlobal
		Name string
	}
)

// Run detaches the tag from the selected objects, on the collector.
func (t *CmdObjectCollectorTagDetach) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return commoncmd.ObjectsAPIDo(mergedSelector, t.OptsGlobal.OptsGlobal, commoncmd.ObjectCollectorTagDetach(t.Name))
}
