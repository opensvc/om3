package commands

import (
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdObjectTakeover struct {
		OptsGlobal
		OptsAsync
		OptsLock
	}
)

func (t *CmdObjectTakeover) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	hn := hostname.Hostname()
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocal(t.Local),
		objectaction.WithFormat(t.Format),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget("placed@"+hn),
		objectaction.WithAsyncWatch(t.Watch),
	).Do()
}
