package commands

import (
	"context"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectComplianceAttachModuleset struct {
		OptsGlobal
		Moduleset string
	}
)

func (t *CmdObjectComplianceAttachModuleset) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithServer(t.Server),
		objectaction.WithRemoteAction("compliance attach moduleset"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Output,
		}),
		objectaction.WithLocalRun(func(ctx context.Context, p naming.Path) (interface{}, error) {
			if o, err := object.NewSvc(p); err != nil {
				return nil, err
			} else {
				comp, err := o.NewCompliance()
				if err != nil {
					return nil, err
				}
				return nil, comp.AttachModuleset(t.Moduleset)
			}
		}),
	).Do()
}
