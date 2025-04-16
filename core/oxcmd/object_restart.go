package oxcmd

import (
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectRestart struct {
		OptsGlobal
		commoncmd.OptsAsync
		commoncmd.OptsLock
		commoncmd.OptsResourceSelector
		commoncmd.OptTo
		NodeSelector    string
		Force           bool
		DisableRollback bool
	}
)

func (t *CmdObjectRestart) Run(selector, kind string) error {
	mergedSelector := commoncmd.MergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRID(t.RID),
		objectaction.WithTag(t.Tag),
		objectaction.WithSubset(t.Subset),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithAsyncTarget("restarted"),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
	).Do()
}
