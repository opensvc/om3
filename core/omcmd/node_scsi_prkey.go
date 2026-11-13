package omcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	CmdNodePRKey struct {
		OptsGlobal
		NodeSelector string
	}
)

// Run prints the key of this node, or with a node selector lists the keys
// of the nodes selected, as their daemons announce them.
func (t *CmdNodePRKey) Run() error {
	if t.NodeSelector != "" {
		return commoncmd.NodePRKeys(t.NodeSelector, t.OptsGlobal.OptsGlobal)
	}
	if err := nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (any, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return n.PRKey()
		}),
	).Do(); err != nil {
		return err
	}
	// The key is on stdout by now. Whether it is this node's alone is
	// the other half of the answer, and the exit code carries it.
	return commoncmd.CheckPRKeyUniqueness()
}
