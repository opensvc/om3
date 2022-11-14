package commands

import (
	"opensvc.com/opensvc/core/nodeaction"
)

type CmdNodeAbort struct {
	OptsGlobal
	OptsAsync
}

func (t *CmdNodeAbort) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithRemoteAction("abort"),
		nodeaction.WithAsyncTarget("aborted"),
		nodeaction.WithAsyncWatch(t.Watch),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocal(t.Local),
	).Do()
}
