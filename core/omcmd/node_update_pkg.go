package omcmd

import (
	"context"

	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	CmdNodeUpdatePkg struct {
		OptsGlobal
		DryRun bool
	}
)

func (t *CmdNodeUpdatePkg) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return nil, n.UpdatePkg(context.Background(), t.DryRun)
		}),
	).Do()
}
