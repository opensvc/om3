package commands

import (
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	CmdKeystoreAdd struct {
		OptsGlobal
		OptsLock
		Key   string
		From  string
		Value string
	}
)

func (t *CmdKeystoreAdd) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("add"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"key":   t.Key,
			"from":  t.From,
			"value": t.Value,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			store, err := object.NewKeystore(p)
			if err != nil {
				return nil, err
			}
			switch {
			case t.From != "":
				return nil, store.AddKeyFrom(t.Key, t.From)
			default:
				return nil, store.AddKey(t.Key, []byte(t.Value))
			}
		}),
	).Do()
}
