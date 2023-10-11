package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/goccy/go-json"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/streamlog"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/render"
)

type (
	CmdNodeLogs struct {
		OptsGlobal
		Follow bool
		Filter *[]string
	}
)

func filterMap(l *[]string) map[string]any {
	m := make(map[string]any)
	if l == nil {
		return m
	}
	for _, s := range *l {
		e := strings.SplitN(s, "=", 2)
		m[e[0]] = e[1]
	}
	return m
}

func (t *CmdNodeLogs) backlog(node string) (streamlog.Events, error) {
	events := make(streamlog.Events, 0)
	c, err := client.New(client.WithURL(node))
	if err != nil {
		return nil, err
	}
	resp, err := c.GetNodeBacklogs(context.Background(), &api.GetNodeBacklogsParams{Filter: t.Filter})
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&events); err != nil {
		return nil, err
	}
	return events, nil
}

func (t *CmdNodeLogs) stream(node string) {
	c, err := client.New(client.WithURL(node), client.WithTimeout(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	reader, err := c.NewGetLogs().
		SetFilters(t.Filter).
		GetReader()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer reader.Close()

	for {
		event, err := reader.Read()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			break
		}
		rec, err := streamlog.NewEvent(event.Data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			break
		}
		rec.Render(t.Output)
	}
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
	events := make(streamlog.Events, 0)
	for _, node := range nodes {
		if more, err := t.backlog(node); err != nil {
			fmt.Fprintln(os.Stderr, "backlog fetch error:", err)
		} else {
			events = append(events, more...)
		}
	}
	events.Sort()
	events.Render(t.Output)
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

func (t *CmdNodeLogs) local() error {
	filters := filterMap(t.Filter)
	if events, err := streamlog.GetEventsFromNode(filters); err == nil {
		events.Render(t.Output)
	} else {
		return err
	}
	if t.Follow {
		stream, err := streamlog.GetEventStreamFromNode(filters)
		if err != nil {
			return err
		}
		for event := range stream.Events() {
			event.Render(t.Output)
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
