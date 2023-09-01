package commands

import (
	"github.com/opensvc/om3/core/nodeaction"
)

type CmdClusterFreeze struct {
	OptsGlobal
	OptsAsync
}

func (t *CmdClusterFreeze) Run() error {
	return nodeaction.New(
		nodeaction.WithAsyncTarget("frozen"),
		nodeaction.WithAsyncTime(t.Time),
		nodeaction.WithAsyncWait(t.Wait),
		nodeaction.WithAsyncWatch(t.Watch),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocal(false),
	).Do()
}
