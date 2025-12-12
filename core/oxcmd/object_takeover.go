package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdObjectTakeover struct {
		OptsGlobal
		commoncmd.OptsAsync
		Live bool
	}
)

func (t *CmdObjectTakeover) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	target := instance.MonitorGlobalExpectPlacedAt.String()
	options := instance.MonitorGlobalExpectOptionsPlacedAt{
		Destination: []string{hostname.Hostname()},
		Live:        t.Live,
	}
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget(target),
		objectaction.WithAsyncTargetOptions(options),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
	).Do()
}
