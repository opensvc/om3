package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/slog"
	"opensvc.com/opensvc/util/render"
)

type (
	// NodeLogs is the cobra flag set of the logs command.
	NodeLogs struct {
		OptsGlobal
		Follow bool   `flag:"logs-follow"`
		SID    string `flag:"logs-sid"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodeLogs) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodeLogs) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "logs",
		Aliases: []string{"logs", "log", "lo"},
		Short:   "filter and format logs",
		Run: func(cmd *cobra.Command, args []string) {
			t.run()
		},
	}
}

func (t *NodeLogs) local() error {
	filters := make(map[string]interface{})
	if t.SID != "" {
		filters["sid"] = t.SID
	}
	if events, err := slog.GetEventsFromNode(filters); err == nil {
		events.Render(t.Format)
	} else {
		return err
	}
	if t.Follow {
		if stream, err := slog.GetEventStreamFromNode(filters); err == nil {
			for event := range stream.Events() {
				event.Render(t.Format)
			}
		}
	}
	return nil
}

func (t *NodeLogs) run() {
	var err error
	render.SetColor(t.Color)
	if t.Local {
		err = t.local()
	} else {
		//err = t.remote(t.Nodes)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
