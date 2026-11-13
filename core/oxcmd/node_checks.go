package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdNodeChecks struct {
		OptsGlobal
		NodeSelector string
	}
)

// Run runs the checks of the nodes selected, which push their results to the
// collector, and lists them.
func (t *CmdNodeChecks) Run() error {
	return commoncmd.NodeChecks(t.NodeSelector, t.OptsGlobal.OptsGlobal)
}
