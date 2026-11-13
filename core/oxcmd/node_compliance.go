package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/util/compliance"
)

// doNodeComplianceRun runs the compliance action on the nodes selected,
// through the api: the run of each node is rendered as om renders the run of
// its node, and the exit code is the one of a module, for the runs as a
// whole.
func doNodeComplianceRun(action compliance.Action, opts commoncmd.ComplianceRunOptions, nodeSelector string, g OptsGlobal) error {
	var runs commoncmd.ComplianceRuns
	if err := nodeaction.New(
		nodeaction.WithRemoteNodes(nodeSelector),
		nodeaction.WithDefaultOutput(""),
		nodeaction.WithFormat(g.Output),
		nodeaction.WithSort(g.Sort),
		nodeaction.WithColor(g.Color),
		nodeaction.WithRemoteFunc(commoncmd.NodeComplianceRunRemote(action, opts, &runs)),
	).Do(); err != nil {
		return err
	}
	return runs.Err()
}
