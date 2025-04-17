package omcmd

import (
	"context"

	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/util/xstrings"
)

type (
	CmdObjectComplianceShowModuleset struct {
		OptsGlobal
		Moduleset string
	}
)

func (t *CmdObjectComplianceShowModuleset) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			if o, err := object.NewSvc(p); err != nil {
				return nil, err
			} else {
				comp, err := o.NewCompliance()
				if err != nil {
					return nil, err
				}
				modsets := xstrings.Split(t.Moduleset, ",")
				data, err := comp.GetData(modsets)
				if err != nil {
					return nil, err
				}
				tree := data.ModulesetsTree()
				return tree, nil
			}
		}),
	).Do()
}
