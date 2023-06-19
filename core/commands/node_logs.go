package commands

import (
	"fmt"
	"os"
	"sync"

	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/slog"
	"github.com/opensvc/om3/util/render"
)

type (
	CmdNodeLogs struct {
		OptsGlobal
		Follow bool
		SID    string
	}
)

func (t *CmdNodeLogs) backlog(node string) (slog.Events, error) {
	events := make(slog.Events, 0)
	/*
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
		if err := json.Unmarshal(b, &events); err != nil {
			return nil, err
		}
	*/
	return events, fmt.Errorf("todo")
}

func (t *CmdNodeLogs) stream(node string) {
	/*
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
	*/
}

func (t *CmdNodeLogs) remote() error {
	sel := nodeselector.New(
		t.NodeSelector,
		nodeselector.WithServer(t.Server),
	)
	nodes, err := sel.Expand()
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes to fetch logs from")
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
