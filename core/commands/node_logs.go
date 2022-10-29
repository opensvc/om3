package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/nodeselector"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/slog"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/render"
)

type (
	CmdNodeLogs struct {
		OptsGlobal
		Follow bool
		SID    string
	}
)

func (t *CmdNodeLogs) backlog(node string) (slog.Events, error) {
	c, err := client.New(
		client.WithURL(node),
		client.WithUsername(hostname.Hostname()),
		client.WithPassword(rawconfig.ClusterSection().Secret),
	)
	if err != nil {
		return nil, err
	}
	req := c.NewGetNodeBacklog().SetFilters(t.Filters())
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

func (t *CmdNodeLogs) stream(node string) {
	c, err := client.New(
		client.WithURL(node),
		client.WithUsername(hostname.Hostname()),
		client.WithPassword(rawconfig.ClusterSection().Secret),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	streamer := c.NewGetNodeLog()
	streamer.Filters = t.Filters()
	events, err := streamer.Do()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	for event := range events {
		event.Render(t.Format)
	}
}

func (t *CmdNodeLogs) remote() error {
	sel := nodeselector.New(
		t.NodeSelector,
		nodeselector.WithServer(t.Server),
	)
	nodes := sel.Expand()
	if len(nodes) == 0 {
		return errors.New("no nodes to fetch logs from")
	}
	events := make(slog.Events, 0)
	for _, node := range nodes {
		if more, err := t.backlog(node); err != nil {
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
			t.stream(node)
		}()
	}
	wg.Wait()
	return nil
}

func (t CmdNodeLogs) Filters() map[string]any {
	filters := make(map[string]any)
	if t.SID != "" {
		filters["sid"] = t.SID
	}
	return filters
}

func (t *CmdNodeLogs) local() error {
	filters := t.Filters()
	if events, err := slog.GetEventsFromNode(filters); err == nil {
		events.Render(t.Format)
	} else {
		return err
	}
	if t.Follow {
		stream, err := slog.GetEventStreamFromNode(filters)
		if err != nil {
			return err
		}
		for event := range stream.Events() {
			event.Render(t.Format)
		}
	}
	return nil
}

func (t *CmdNodeLogs) Run() error {
	var err error
	render.SetColor(t.Color)
	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}
	if t.Local {
		err = t.local()
	} else {
		err = t.remote()
	}
	return err
}
