package omcmd

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/compliance"
)

// doNodeComplianceRun runs the compliance action on this node, or with a
// node selector on the nodes selected, through the api. The output is the
// same, the run of each node, and so is the exit code: the one of a module,
// for the runs as a whole.
func doNodeComplianceRun(action compliance.Action, opts commoncmd.ComplianceRunOptions, nodeSelector string, g OptsGlobal) error {
	var runs commoncmd.ComplianceRuns
	if err := nodeaction.New(
		nodeaction.WithRemoteNodes(nodeSelector),
		nodeaction.WithDefaultOutput(""),
		nodeaction.WithFormat(g.Output),
		nodeaction.WithSort(g.Sort),
		nodeaction.WithColor(g.Color),
		nodeaction.WithLocalFunc(func() (any, error) {
			run, err := localNodeComplianceRun(action, opts)
			if err != nil {
				return nil, err
			}
			runs.Add(run)
			return run, nil
		}),
		nodeaction.WithRemoteFunc(commoncmd.NodeComplianceRunRemote(action, opts, &runs)),
	).Do(); err != nil {
		return err
	}
	return runs.Err()
}

func localNodeComplianceRun(action compliance.Action, opts commoncmd.ComplianceRunOptions) (*compliance.Run, error) {
	comp, err := newNodeCompliance()
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

// newNodeCompliance returns the compliance of this node, a client of the
// collector authenticated as the node.
func newNodeCompliance() (*compliance.T, error) {
	n, err := object.NewNode()
	if err != nil {
		return nil, err
	}
	return n.NewCompliance()
}
