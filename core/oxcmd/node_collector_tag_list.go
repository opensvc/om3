package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdNodeCollectorTagList struct {
		OptsGlobal
	}
)

// Run lists the tags of the collector, as it answers them to the node
// serving the request.
func (t *CmdNodeCollectorTagList) Run() error {
	return commoncmd.CollectorTagList(t.OptsGlobal.OptsGlobal)
}
