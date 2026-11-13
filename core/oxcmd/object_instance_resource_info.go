package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/objectaction"
)

type (
	CmdObjectInstanceResourceInfo struct {
		OptsGlobal
		commoncmd.OptsAsync
		commoncmd.OptsResourceSelector
		NodeSelector string

		// Refresh recomputes the resource info cache before reporting it.
		// Without it, the cached key-values are reported as-is.
		Refresh bool
	}
)

func (t *CmdObjectInstanceResourceInfo) Run(kind string) error {
	if t.Refresh {
		return t.refresh(kind)
	}
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return commoncmd.ObjectResourceInfoList(mergedSelector, t.NodeSelector, t.RID, t.OptsGlobal.OptsGlobal)
}

// refresh asks the nodes holding an instance to recompute their resource info
// cache. Reporting the refreshed key-values to the collector is the collector
// speaker's job, driven by the InstanceResourceInfoUpdated signal.
func (t *CmdObjectInstanceResourceInfo) refresh(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithOutput(t.Output),
		objectaction.WithSort(t.Sort),
		objectaction.WithColor(t.Color),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithAsyncFollow(t.Follow),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteFunc(commoncmd.ObjectResourceInfoRefreshRemote(t.RID)),
	).Do()
}
