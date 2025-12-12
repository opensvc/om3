package oxcmd

import (
	"github.com/opensvc/om3/v3/core/nodeaction"
)

type (
	CmdNodeCollectorTagList struct {
		OptsGlobal
	}
)

func (t *CmdNodeCollectorTagList) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
	).Do()
}
