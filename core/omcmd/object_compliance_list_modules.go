package omcmd

import (
	"context"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/util/compliance"
)

type (
	CmdObjectComplianceListModules struct {
		OptsGlobal
	}
)

func (t *CmdObjectComplianceListModules) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithColor(t.Color),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
		objectaction.WithOutput(t.Output),
		objectaction.WithSort(t.Sort),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			if _, err := object.NewSvc(p); err != nil {
				return nil, err
			}
			// The modules installed on the node: no collector is
			// asked, none is needed.
			return compliance.New().ListModuleNames()
		}),
	).Do()
}
