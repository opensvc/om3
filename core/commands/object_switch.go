package commands

import (
	"opensvc.com/opensvc/core/objectaction"
)

type (
	CmdObjectSwitch struct {
		OptsGlobal
		OptsAsync
		OptsLock
		To string
	}
)

func (t *CmdObjectSwitch) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	target := "placed@"
	if t.To != "" {
		target += t.To
	}
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocal(t.Local),
		objectaction.WithFormat(t.Format),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget(target),
		objectaction.WithAsyncWatch(t.Watch),
	).Do()
}
