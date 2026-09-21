package omcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectResize struct {
		OptsGlobal
		commoncmd.OptsAsync
		Size string
	}
)

// Run asks the daemons to grow the object.
//
// The size is handed over as the user wrote it. Resolving it against the size
// already configured, checking the namespace has room for it in the pool, and
// writing it where every node reads it are the daemon's, on the node holding
// the object: this command has no say in whether the grow is allowed, and a
// client reaching the api directly must not either.
func (t *CmdObjectResize) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	body := api.PostObjectActionResize{}
	if t.Size != "" {
		body.Size = &t.Size
	}
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget("resized"),
		objectaction.WithAsyncTargetOptions(body),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithSort(t.Sort),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
	).Do()
}
