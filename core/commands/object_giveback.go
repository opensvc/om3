package commands

import (
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectGiveback struct {
		OptsGlobal
		OptsAsync
		OptsLock
	}
)

func (t *CmdObjectGiveback) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocal(t.Local),
		objectaction.WithFormat(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget("placed"),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
	).Do()
}
