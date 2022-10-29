package commands

import (
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	CmdKeystoreRemove struct {
		OptsGlobal
		Key string
	}
)

func (t *CmdKeystoreRemove) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("keys"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"key": t.Key,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			store, err := object.NewKeystore(p)
			if err != nil {
				return nil, err
			}
			return nil, store.RemoveKey(t.Key)
		}),
	).Do()
}
