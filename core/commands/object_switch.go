package commands

import (
	"strings"

	"opensvc.com/opensvc/core/instance"
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
		objectaction.WithFormat(t.Format),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget(target),
		objectaction.WithAsyncTargetOptions(options),
		objectaction.WithAsyncWatch(t.Watch),
	).Do()
}
