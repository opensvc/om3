package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/entrypoints/monitor"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdObjectMonitor is the cobra flag set of the monitor command.
	CmdObjectMonitor struct {
		Global object.OptsGlobal
		Watch  bool `flag:"watch"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectMonitor) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	object.InstallFlags(cmd, t)
}

func (t *CmdObjectMonitor) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "monitor",
		Aliases: []string{"mon", "moni", "monit", "monito"},
		Short:   "Print selected service and instance status summary",
		Long:    monitor.CmdLong,
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectMonitor) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	cli, err := client.New(client.URL(t.Global.Server))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	m := monitor.New()
	m.SetColor(t.Global.Color)
	m.SetFormat(t.Global.Format)
	m.SetSections([]string{"objects"})

	if t.Watch {
		getter, _ := client.NewGetEvents(*cli, client.WithSelector(mergedSelector))
		m.DoWatch(getter, os.Stdout)
	} else {
		getter, _ := client.NewGetDaemonStatusB(*cli, client.WithSelector(mergedSelector))
		m.Do(getter, os.Stdout)
	}
}
