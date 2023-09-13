package commands

import (
	"context"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/util/key"
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
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("eval"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"kw":          t.Keyword,
			"impersonate": t.Impersonate,
			"eval":        true,
		}),
		objectaction.WithLocalRun(func(ctx context.Context, p path.T) (interface{}, error) {
			c, err := object.NewConfigurer(p)
			if err != nil {
				return nil, err
			}
			return c.EvalAs(key.Parse(t.Keyword), t.Impersonate)
		}),
	).Do()
}
