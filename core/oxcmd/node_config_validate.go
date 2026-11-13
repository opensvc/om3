package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdNodeConfigValidate struct {
		OptsGlobal
		NodeSelector string
	}
)

// Run validates the configuration of the selected nodes, each on the node.
func (t *CmdNodeConfigValidate) Run() error {
	return commoncmd.NodeConfigValidate(t.NodeSelector, t.OptsGlobal.OptsGlobal)
}
