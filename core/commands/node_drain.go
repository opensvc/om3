package commands

import (
	"github.com/opensvc/om3/core/nodeaction"
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
		nodeaction.WithAsyncTime(t.Time),
		nodeaction.WithAsyncWait(t.Wait),
		nodeaction.WithAsyncWatch(t.Watch),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocal(t.Local),
	).Do()
}
