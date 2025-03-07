package omcmd

import (
	"strings"

	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectSwitch struct {
		OptsGlobal
		commoncmd.OptsAsync
		commoncmd.OptsLock
		To string
	}
)

func (t *CmdObjectSwitch) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	target := instance.MonitorGlobalExpectPlacedAt.String()
	options := instance.MonitorGlobalExpectOptionsPlacedAt{}
	if t.To != "" {
		options.Destination = strings.Split(t.To, ",")
	} else {
		options.Destination = []string{}
	}
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocal(t.Local),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget(target),
		objectaction.WithAsyncTargetOptions(options),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
	).Do()
}
