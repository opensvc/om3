package commands

import (
	"context"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/path"
)

type (
	CmdObjectComplianceListRuleset struct {
		OptsGlobal
		Ruleset string
	}
)

func (t *CmdObjectComplianceListRuleset) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithServer(t.Server),
		objectaction.WithRemoteAction("compliance env"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Output,
		}),
		objectaction.WithLocalRun(func(ctx context.Context, p path.T) (interface{}, error) {
			if o, err := object.NewSvc(p); err != nil {
				return nil, err
			} else {
				comp, err := o.NewCompliance()
				if err != nil {
					return nil, err
				}
				return comp.ListRulesets(t.Ruleset)
			}
		}),
	).Do()
}
