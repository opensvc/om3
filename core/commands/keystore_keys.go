package commands

import (
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	CmdKeystoreKeys struct {
		OptsGlobal
		Match string
	}
)

func (t *CmdKeystoreKeys) Run(selector, kind string) error {
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
			"match": t.Match,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			store, err := object.NewKeystore(p)
			if err != nil {
				return nil, err
			}
			return store.MatchingKeys(t.Match)
		}),
	).Do()
}
