package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/entrypoints/monitor"
	"opensvc.com/opensvc/core/flag"
)

type (
	// CmdObjectMonitor is the cobra flag set of the monitor command.
	CmdObjectMonitor struct {
		OptsGlobal
		Watch bool `flag:"watch"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectMonitor) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectMonitor) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "monitor",
		Aliases: []string{"mon", "moni", "monit", "monito"},
		Short:   "print selected service and instance status summary",
		Long:    monitor.CmdLong,
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectMonitor) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	cli, err := client.New(client.WithURL(t.Server))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	m := monitor.New()
	m.SetColor(t.Color)
	m.SetFormat(t.Format)
	m.SetSections([]string{"objects"})

	if t.Watch {
		getter := cli.NewGetEvents().SetSelector(mergedSelector)
		if err := m.DoWatch(getter, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		getter := cli.NewGetDaemonStatus().SetSelector(mergedSelector)
		if err := m.Do(getter, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
