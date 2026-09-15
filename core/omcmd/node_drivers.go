package omcmd

import (
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	CmdNodeDrivers struct {
		Color  string
		Output string
		Sort   string
	}
)

func (t *CmdNodeDrivers) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return n.Drivers()
		}),
	).Do()
}
