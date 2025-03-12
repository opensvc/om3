package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/util/keystore"
	"github.com/opensvc/om3/util/uri"
)

type (
	CmdKeystoreChange struct {
		OptsGlobal
		Key   string
		From  *string
		Value *string
	}
)

func (t *CmdKeystoreChange) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			store, err := object.NewKeystore(p)
			if err != nil {
				return nil, err
			}
			if t.Value != nil {
				return nil, store.ChangeKey(t.Key, []byte(*t.Value))
			}
			if t.From == nil {
				return nil, fmt.Errorf("value or value source mut be specified for a change action")
			}
			m, err := uri.ReadAllFrom(*t.From)
			if err != nil {
				return nil, err
			}
			for path, b := range m {
				k, err := keystore.FileToKey(path, t.Key, *t.From)
				if err != nil {
					return nil, err
				}

				if err := store.TransactionChangeKey(k, b); err != nil {
					return nil, err
				}
			}
			return nil, store.Config().CommitInvalid()
		}),
	).Do()
}
