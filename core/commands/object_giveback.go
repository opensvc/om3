package commands

import (
	"opensvc.com/opensvc/core/objectaction"
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
		objectaction.WithFormat(t.Format),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget("placed"),
		objectaction.WithAsyncWatch(t.Watch),
	).Do()
}
