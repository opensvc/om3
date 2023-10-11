package commands

import (
	"context"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/objectlogger"
	"github.com/opensvc/om3/util/key"
)

type (
	CmdObjectGet struct {
		OptsGlobal
		Eval    bool
		Keyword string
	}
)

func (t *CmdObjectGet) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("get"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"kw": t.Keyword,
		}),
		objectaction.WithLocalRun(func(ctx context.Context, p naming.Path) (interface{}, error) {
			logger := objectlogger.New(p,
				objectlogger.WithColor(t.Color != "no"),
				objectlogger.WithConsoleLog(t.Log != ""),
				objectlogger.WithLogFile(true),
			)
			c, err := object.NewConfigurer(p, object.WithLogger(logger))
			if err != nil {
				return nil, err
			}
			if t.Eval {
				return c.Eval(key.Parse(t.Keyword))
			} else {
				return c.Get(key.Parse(t.Keyword))
			}
		}),
	).Do()
}
