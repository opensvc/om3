package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/objectaction"
)

type (
	CmdObjectStop struct {
		OptsGlobal
		commoncmd.OptsAsync
		InterruptSyncs bool
	}
)

func (t *CmdObjectStop) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithOutput(t.Output),
		objectaction.WithSort(t.Sort),
		objectaction.WithColor(t.Color),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
		objectaction.WithAsyncTarget("stopped"),
		objectaction.WithAsyncTargetOptions(instance.MonitorGlobalExpectOptionsStopped{InterruptSyncs: t.InterruptSyncs}),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithAsyncFollow(t.Follow),
	).Do()
}
