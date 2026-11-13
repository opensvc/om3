package omcmd

import (
	"context"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/objectaction"
)

type (
	CmdObjectComplianceEnv struct {
		OptsGlobal
		Moduleset    string
		Module       string
		NodeSelector string
	}
)

func (t *CmdObjectComplianceEnv) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithColor(t.Color),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
		objectaction.WithOutput(t.Output),
		objectaction.WithSort(t.Sort),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithDefaultOutput(""),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (any, error) {
			comp, err := newObjectCompliance(p)
			if err != nil {
				return nil, err
			}
			run := comp.NewRun()
			run.SetModulesetsExpr(t.Moduleset)
			run.SetModulesExpr(t.Module)
			return run.Env()
		}),
		objectaction.WithRemoteFunc(commoncmd.ObjectComplianceEnvRemote(t.Moduleset, t.Module)),
	).Do()
}
