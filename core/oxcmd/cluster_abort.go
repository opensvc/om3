package oxcmd

import (
	"github.com/opensvc/om3/core/nodeaction"
)

type CmdClusterAbort struct {
	OptsGlobal
	OptsAsync
}

func (t *CmdClusterAbort) Run() error {
	return nodeaction.New(
		nodeaction.WithAsyncTarget("aborted"),
		nodeaction.WithAsyncWatch(t.Watch),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocal(t.Local),
	).Do()
}
