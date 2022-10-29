package commands

import (
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/key"
)

type (
	CmdObjectEval struct {
		OptsGlobal
		Keyword     string
		Impersonate string
	}
)

func (t *CmdObjectEval) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("eval"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"kw":          t.Keyword,
			"impersonate": t.Impersonate,
			"eval":        true,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			c, err := object.NewConfigurer(p)
			if err != nil {
				return nil, err
			}
			return c.EvalAs(key.Parse(t.Keyword), t.Impersonate)
		}),
	).Do()
}
