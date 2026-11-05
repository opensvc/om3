package omcmd

import (
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeaction"
)

type (
	CmdNodeCollectorTagAttach struct {
		OptsGlobal
		Name       string
		AttachData *string
	}
)

func (t *CmdNodeCollectorTagAttach) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			return collectorTagAttach(t.Name, naming.Path{}, t.AttachData)
		}),
	).Do()
}
