package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/util/compliance"
)

// doObjectComplianceRun runs the compliance action of the selected objects
// on their instances of the nodes selected, through the api: the run of
// each instance is rendered as om renders a run on its node, and the exit
// code is the one of a module, for the runs as a whole.
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
		objectaction.WithRemoteFunc(commoncmd.ObjectComplianceRunRemote(action, opts, &runs)),
	).Do(); err != nil {
		return err
	}
	return runs.Err()
}
