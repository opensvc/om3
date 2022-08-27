package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/slog"
	"opensvc.com/opensvc/util/render"
	"opensvc.com/opensvc/util/xmap"
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

func (t CmdObjectLogs) Filters() map[string]any {
	filters := make(map[string]any)
	if t.SID != "" {
		filters["sid"] = t.SID
	}
	return filters
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

func (t *CmdObjectLogs) backlog(node string, paths path.L) (slog.Events, error) {
	c, err := client.New(
		client.WithURL(node),
		client.WithPassword(rawconfig.ClusterSection().Secret),
	)
	if err != nil {
		return nil, err
	}
	req := c.NewGetObjectsBacklog()
	req.Filters = t.Filters()
	req.Paths = paths
	b, err := req.Do()
	if err != nil {
		return nil, err
	}
	events := make(slog.Events, 0)
	if err := json.Unmarshal(b, &events); err != nil {
		return nil, err
	}
	return events, nil
}

func (t *CmdObjectLogs) stream(node string, paths path.L) {
	c, err := client.New(
		client.WithURL(node),
		client.WithPassword(rawconfig.ClusterSection().Secret),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	streamer := c.NewGetObjectsLog()
	streamer.Filters = t.Filters()
	streamer.Paths = paths
	events, err := streamer.Do()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	for event := range events {
		event.Render(t.Format)
	}
}

func nodesFromPaths(paths path.L) []string {
	m := make(map[string]any)
	for _, p := range paths {
		o, err := object.NewCore(p, object.WithVolatile(true))
		if err != nil {
			continue
		}
		for _, node := range o.Nodes() {
			m[node] = nil
		}
	}
	return xmap.Keys(m)
}

func (t *CmdObjectLogs) remote(selStr string) error {
	sel := objectselector.NewSelection(
		selStr,
		objectselector.SelectionWithLocal(true),
	)
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	nodes := nodesFromPaths(paths)
	filters := make(map[string]interface{})
	if t.SID != "" {
		filters["sid"] = t.SID
	}
	events := make(slog.Events, 0)
	for _, node := range nodes {
		if more, err := t.backlog(node, paths); err != nil {
			fmt.Fprintln(os.Stderr, "backlog fetch error:", err)
		} else {
			events = append(events, more...)
		}
	}
	events.Sort()
	events.Render(t.Format)

	if !t.Follow {
		return nil
	}
	var wg sync.WaitGroup
	wg.Add(len(nodes))
	for _, node := range nodes {
		go func() {
			defer wg.Done()
			t.stream(node, paths)
		}()
	}
	wg.Wait()
	return nil
}

func (t *CmdObjectLogs) local(selStr string) error {
	sel := objectselector.NewSelection(
		selStr,
		objectselector.SelectionWithLocal(true),
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
		stream, err := slog.GetEventStreamFromObjects(paths, filters)
		if err != nil {
			return err
		}
		for event := range stream.Events() {
			event.Render(t.Format)
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
		err = t.remote(mergedSelector)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
