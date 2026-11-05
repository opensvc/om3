package omcmd

import (
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeaction"
)

type (
	CmdNodeCollectorTagShow struct {
		OptsGlobal
		Verbose bool
	}
)

func (t *CmdNodeCollectorTagShow) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			return collectorTagShow(naming.Path{}, t.Verbose)
		}),
	).Do()
}
