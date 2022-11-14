package commands

import (
	"opensvc.com/opensvc/core/nodeaction"
)

type CmdNodeDrain struct {
	OptsGlobal
	OptsAsync
}

func (t *CmdNodeDrain) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithRemoteAction("drain"),
		nodeaction.WithAsyncTarget("drained"),
		nodeaction.WithAsyncWatch(t.Watch),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocal(t.Local),
	).Do()
}
