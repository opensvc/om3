package commands

import (
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type CmdNodeFreeze struct {
	OptsGlobal
	OptsAsync
}

func (t *CmdNodeFreeze) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithRemoteAction("freeze"),
		nodeaction.WithAsyncTarget("frozen"),
		nodeaction.WithAsyncWatch(t.Watch),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return nil, n.Freeze()
		}),
	).Do()
}
