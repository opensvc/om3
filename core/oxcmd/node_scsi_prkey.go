package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdNodePRKey struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodePRKey) Run() error {
	selector := t.NodeSelector
	if selector == "" {
		selector = "*"
	}
	return commoncmd.NodePRKeys(selector, t.OptsGlobal.OptsGlobal)
}
