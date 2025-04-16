package omcmd

import (
	"context"

	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/util/datastore"
	"github.com/opensvc/om3/util/uri"
)

type (
	CmdObjectKeyAdd struct {
		OptsGlobal
		commoncmd.OptsLock
		Name  string
		From  *string
		Value *string
	}
)

func (t *CmdObjectKeyAdd) Run(selector, kind string) error {
	mergedSelector := commoncmd.MergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			store, err := object.NewDataStore(p)
			if err != nil {
				return nil, err
			}
			if t.Value != nil {
				return nil, store.AddKey(t.Name, []byte(*t.Value))
			}
			if t.From == nil {
				return nil, store.AddKey(t.Name, []byte{})
			}
			m, err := uri.ReadAllFrom(*t.From)
			if err != nil {
				return nil, err
			}
			for path, b := range m {
				k, err := datastore.FileToKey(path, t.Name, *t.From)
				if err != nil {
					return nil, err
				}
				if err := store.TransactionAddKey(k, b); err != nil {
					return nil, err
				}
			}
			return nil, store.Config().CommitInvalid()
		}),
	).Do()
}
