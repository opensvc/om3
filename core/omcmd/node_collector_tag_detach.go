package omcmd

import (
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeaction"
)

type (
	CmdNodeCollectorTagDetach struct {
		OptsGlobal
		Name string
	}
)

func (t *CmdNodeCollectorTagDetach) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			return collectorTagDetach(t.Name, naming.Path{})
		}),
	).Do()
}
