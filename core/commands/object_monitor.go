package commands

import (
	"fmt"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/api/daemon/status"
	"opensvc.com/opensvc/core/api/getevent"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/entrypoints/monitor"
	"opensvc.com/opensvc/core/entrypoints/omonitor"
	"os"
)

type (
	// CmdObjectMonitor is the cobra flag set of the monitor command.
	CmdObjectMonitor struct {
		flagSetGlobal
		flagSetObject
		Watch bool
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectMonitor) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	t.flagSetGlobal.init(cmd)
	t.flagSetObject.init(cmd)
	cmd.Flags().BoolVarP(&t.Watch, "watch", "w", false, "Watch the monitor changes")
}

func (t *CmdObjectMonitor) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "monitor",
		Aliases: []string{"mon", "moni", "monit", "monito"},
		Short:   "Print selected service and instance status summary",
		Long:    omonitor.CmdLong,
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectMonitor) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	cli, err := client.New().SetURL(t.Server).Configure()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	m := monitor.New()
	m.SetColor(t.Color)
	m.SetFormat(t.Format)
	m.SetSections([]string{"objects"})

	if t.Watch {
		getter := getevent.New(*cli, mergedSelector, true)
		m.DoWatch(getter, os.Stdout)
	} else {
		getter := status.New(*cli, mergedSelector)
		m.Do(getter, os.Stdout)
	}
}
