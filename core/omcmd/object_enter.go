package omcmd

import (
	"context"

	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectEnter struct {
		ObjectSelector string
		RID            string
	}

	enterer interface {
		Enter(ctx context.Context, rid string) error
	}
)

func (t *CmdObjectEnter) Run(selector, kind string) error {
	mergedSelector := commoncmd.MergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			o, err := object.NewActor(p)
			if err != nil {
				return nil, err
			}
			return nil, o.Enter(ctx, t.RID)
		}),
	).Do()
}
