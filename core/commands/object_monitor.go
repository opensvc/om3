package commands

import (
	"os"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/entrypoints/monitor"
)

type (
	CmdObjectMonitor struct {
		OptsGlobal
		Watch bool
	}
)

func (t *CmdObjectMonitor) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	cli, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}

	m := monitor.New()
	m.SetColor(t.Color)
	m.SetFormat(t.Format)
	m.SetSections([]string{"objects"})
	m.SetSelector(mergedSelector)

	if t.Watch {
		evGetter := cli.NewGetEvents().SetSelector(mergedSelector)
		statusGetter := cli.NewGetDaemonStatus().SetSelector(mergedSelector)
		if err := m.DoWatch(statusGetter, evGetter, os.Stdout); err != nil {
			return err
		}
	} else {
		getter := cli.NewGetDaemonStatus().SetSelector(mergedSelector)
		if err := m.Do(getter, os.Stdout); err != nil {
			return err
		}
	}
	return nil
}
