package commands

import (
	"context"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
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
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewActor(p)
			if err != nil {
				return nil, err
			}
			ctx := context.Background()
			return nil, o.Enter(ctx, t.RID)
		}),
	).Do()
}
