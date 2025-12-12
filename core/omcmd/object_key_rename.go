package omcmd

import (
	"context"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
)

type (
	CmdObjectKeyRename struct {
		OptsGlobal
		Name string
		To   string
	}
)

func (t *CmdObjectKeyRename) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			store, err := object.NewDataStore(p)
			if err != nil {
				return nil, err
			}
			return nil, store.RenameKey(t.Name, t.To)
		}),
	).Do()
}
