package oxcmd

import (
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectAbort struct {
		OptsGlobal
		commoncmd.OptsAsync
	}
)

func (t *CmdObjectAbort) Run(selector, kind string) error {
	mergedSelector := commoncmd.MergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget("aborted"),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
	).Do()
}
