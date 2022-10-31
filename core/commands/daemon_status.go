package commands

import (
	"os"

	"opensvc.com/opensvc/core/monitor"
)

type (
	CmdDaemonStatus struct {
		OptsGlobal
		Watch bool
	}
)

func (t *CmdDaemonStatus) Run() error {
	m := monitor.New()
	m.SetColor(t.Color)
	m.SetFormat(t.Format)

	cli, err := newClient(t.Server)
	if err != nil {
		return err
	}
	if t.Watch {
		statusGetter := cli.NewGetDaemonStatus().SetSelector(t.ObjectSelector)
		eventsGetter := cli.NewGetEvents().SetSelector(t.ObjectSelector)
		return m.DoWatch(statusGetter, eventsGetter, os.Stdout)
	} else {
		getter := cli.NewGetDaemonStatus().SetSelector(t.ObjectSelector)
		return m.Do(getter, os.Stdout)
	}
}
