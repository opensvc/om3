package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/objectaction"
)

type (
	CmdObjectUnfreeze struct {
		OptsGlobal
		commoncmd.OptsAsync
		NodeSelector string
	}
)

func (t *CmdObjectUnfreeze) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget("unfrozen"),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithRemoteNodes(t.NodeSelector),
	).Do()
}
