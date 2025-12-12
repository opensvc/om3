package oxcmd

import (
	"github.com/opensvc/om3/v3/core/nodeaction"
)

type (
	CmdNodeCollectorTagCreate struct {
		OptsGlobal
		Name    string
		Data    *string
		Exclude *string
	}
)

func (t *CmdNodeCollectorTagCreate) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
	).Do()
}
