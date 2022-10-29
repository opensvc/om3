package commands

import (
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/key"
)

type (
	CmdObjectGet struct {
		OptsGlobal
		Keyword string
	}
)

func (t *CmdObjectGet) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("get"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"kw": t.Keyword,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			c, err := object.NewConfigurer(p)
			if err != nil {
				return nil, err
			}
			return c.Get(key.Parse(t.Keyword))
		}),
	).Do()
}
