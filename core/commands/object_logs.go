package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/slog"
	"opensvc.com/opensvc/util/render"
)

type (
	// CmdObjectLogs is the cobra flag set of the logs command.
	CmdObjectLogs struct {
		OptsGlobal
		Follow bool   `flag:"logs-follow"`
		SID    string `flag:"logs-sid"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectLogs) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectLogs) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "logs",
		Aliases: []string{"logs", "log", "lo"},
		Short:   "filter and format logs",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectLogs) local(selStr string) error {
	sel := object.NewSelection(
		selStr,
		object.SelectionWithLocal(true),
	)
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	filters := make(map[string]interface{})
	if t.SID != "" {
		filters["sid"] = t.SID
	}
	if events, err := slog.GetEventsFromObjects(paths, filters); err == nil {
		events.Render(t.Format)
	} else {
		return err
	}
	if t.Follow {
		if stream, err := slog.GetEventStreamFromObjects(paths, filters); err == nil {
			for event := range stream.Events() {
				event.Render(t.Format)
			}
		}
	}
	return nil
}

func (t *CmdObjectLogs) run(selector *string, kind string) {
	var err error
	render.SetColor(t.Color)
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "**")
	if t.Local {
		err = t.local(mergedSelector)
	} else {
		//err = t.remote(mergedSelector)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
