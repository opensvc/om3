package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/util/compliance"
)

// doObjectComplianceRun runs the compliance action of the selected objects
// on this node, or with a node selector on their instances of the nodes
// selected, through the api. The output is the same, the run of each
// instance, and so is the exit code: the one of a module, for the runs as a
// whole.
func doObjectComplianceRun(kind string, action compliance.Action, opts commoncmd.ComplianceRunOptions, nodeSelector string, g OptsGlobal) error {
	var runs commoncmd.ComplianceRuns
	mergedSelector := commoncmd.MergeSelector("", g.ObjectSelector, kind, "")
	if err := objectaction.New(
		objectaction.WithColor(g.Color),
		objectaction.WithIgnoreNotFound(g.IgnoreNotFound),
		objectaction.WithOutput(g.Output),
		objectaction.WithSort(g.Sort),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(nodeSelector),
		objectaction.WithDefaultOutput(""),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (any, error) {
			run, err := localObjectComplianceRun(p, action, opts)
			if err != nil {
				return nil, err
			}
			runs.Add(run)
			return run, nil
		}),
		objectaction.WithRemoteFunc(commoncmd.ObjectComplianceRunRemote(action, opts, &runs)),
	).Do(); err != nil {
		return err
	}
	return runs.Err()
}

func localObjectComplianceRun(p naming.Path, action compliance.Action, opts commoncmd.ComplianceRunOptions) (*compliance.Run, error) {
	comp, err := newObjectCompliance(p)
	if err != nil {
		return nil, err
	}
	run := comp.NewRun()
	run.SetModulesetsExpr(opts.Modulesets)
	run.SetModulesExpr(opts.Modules)
	run.SetForce(opts.Force)
	run.SetAttach(opts.Attach)
	switch action {
	case compliance.ActionAuto:
		err = run.Auto()
	case compliance.ActionCheck:
		err = run.Check()
	case compliance.ActionFix:
		err = run.Fix()
	case compliance.ActionFixable:
		err = run.Fixable()
	default:
		err = fmt.Errorf("unexpected compliance action %s", action)
	}
	return run, err
}

// newObjectCompliance returns the compliance of the object, a client of the
// collector authenticated as this node, asking for the object.
func newObjectCompliance(p naming.Path) (*compliance.T, error) {
	o, err := object.NewSvc(p)
	if err != nil {
		return nil, err
	}
	return o.NewCompliance()
}
