package omcmd

import (
	"context"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectKeyInstall struct {
		OptsGlobal
		NodeSelector string
		Key          string
	}
)

func (t *CmdObjectKeyInstall) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			store, err := object.NewKVStore(p)
			if err != nil {
				return nil, err
			}
			return nil, store.InstallKey(t.Key)
		}),
	).Do()
}
