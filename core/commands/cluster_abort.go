package commands

import (
	"github.com/opensvc/om3/core/nodeaction"
)

type CmdClusterAbort struct {
	OptsGlobal
	OptsAsync
}

func (t *CmdClusterAbort) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithRemoteAction("abort"),
		nodeaction.WithAsyncTarget("aborted"),
		nodeaction.WithAsyncWatch(t.Watch),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocal(t.Local),
	).Do()
}
