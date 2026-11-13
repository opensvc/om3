package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectCollectorTagCreate struct {
		OptsGlobal
		Name    string
		Data    *string
		Exclude *string
	}
)

// Run creates the tag on the collector, as the node serving the request.
func (t *CmdObjectCollectorTagCreate) Run(kind string) error {
	return commoncmd.CollectorTagCreate(t.Name, t.Data, t.Exclude)
}
