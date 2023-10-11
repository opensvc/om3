package commands

import (
	"context"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/objectlogger"
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
		objectaction.WithLocalRun(func(ctx context.Context, p naming.Path) (interface{}, error) {
			logger := objectlogger.New(p,
				objectlogger.WithLogFile(true),
			)
			o, err := object.NewActor(p, object.WithLogger(logger))
			if err != nil {
				return nil, err
			}
			return nil, o.Enter(ctx, t.RID)
		}),
	).Do()
}
