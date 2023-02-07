package commands

import (
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type CmdNodeUnfreeze struct {
	OptsGlobal
	OptsAsync
}

func (t *CmdNodeUnfreeze) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithRemoteAction("unfreeze"),
		nodeaction.WithAsyncTarget("thawed"),
		nodeaction.WithAsyncTime(t.Time),
		nodeaction.WithAsyncWait(t.Wait),
		nodeaction.WithAsyncWatch(t.Watch),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return nil, n.Unfreeze()
		}),
	).Do()
}
