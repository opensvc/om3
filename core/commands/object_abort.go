package commands

import "github.com/opensvc/om3/core/objectaction"

type (
	CmdObjectAbort struct {
		OptsGlobal
		OptsAsync
	}
)

func (t *CmdObjectAbort) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocal(t.Local),
		objectaction.WithFormat(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget("aborted"),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithProgress(!t.Quiet && t.Log == ""),
	).Do()
}
