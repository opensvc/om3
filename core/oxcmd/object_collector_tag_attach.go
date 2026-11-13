package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectCollectorTagAttach struct {
		OptsGlobal
		Name       string
		AttachData *string
	}
)

// Run attaches the tag to the selected objects, on the collector.
func (t *CmdObjectCollectorTagAttach) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return commoncmd.ObjectsAPIDo(mergedSelector, t.OptsGlobal.OptsGlobal, commoncmd.ObjectCollectorTagAttach(t.Name, t.AttachData))
}
