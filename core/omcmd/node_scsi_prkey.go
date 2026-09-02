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

func (t *CmdNodePRKey) Run() error {
	if err := nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithRemoteNodes(t.NodeSelector),
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
