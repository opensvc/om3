package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/objectaction"
)

type (
	CmdObjectInstanceDelete struct {
		OptsGlobal
		commoncmd.OptsAsync
		NodeSelector string
	}
)

func (t *CmdObjectInstanceDelete) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithColor(t.Color),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteFunc(commoncmd.ObjectInstanceDeleteRemoteFunc),
	).Do()
}
