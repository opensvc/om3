package commands

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/goccy/go-json"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/streamlog"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/render"
	"github.com/opensvc/om3/util/xmap"
)

type (
	CmdObjectLogs struct {
		OptsGlobal
		Follow bool
		Filter *[]string
	}
)

func (t *CmdObjectLogs) backlog(node string, paths naming.Paths) (streamlog.Events, error) {
	events := make(streamlog.Events, 0)
	c, err := client.New(client.WithURL(node))
	if err != nil {
		return nil, err
	}
	resp, err := c.GetInstancesBacklogs(context.Background(), &api.GetInstancesBacklogsParams{Filter: t.Filter, Paths: paths.StrSlice()})
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&events); err != nil {
		return nil, err
	}
	return events, nil
}

func (t *CmdObjectLogs) stream(node string, paths naming.Paths) {
	c, err := client.New(client.WithURL(node), client.WithTimeout(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	l := paths.StrSlice()
	reader, err := c.NewGetLogs().
		SetFilters(t.Filter).
		SetPaths(&l).
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

func nodesFromPath(p naming.Path) ([]string, error) {
	o, err := object.NewCore(p, object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	return o.Nodes()
}

func nodesFromPaths(paths naming.Paths) ([]string, error) {
	m := make(map[string]any)
	for _, p := range paths {
		nodes, err := nodesFromPath(p)
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			m[node] = nil
		}
	}
	return xmap.Keys(m), nil
}

func (t *CmdObjectLogs) remote(selStr string) error {
	var (
		nodes []string
		err   error
	)
	sel := objectselector.NewSelection(
		selStr,
		objectselector.SelectionWithLocal(true),
	)
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	if t.NodeSelector != "" {
		nodeSelector := nodeselector.New(
			t.NodeSelector,
			nodeselector.WithLocal(true),
		)
		nodes, err = nodeSelector.Expand()
		if err != nil {
			return err
		}
	} else {
		nodes, err = nodesFromPaths(paths)
		if err != nil {
			return err
		}
	}
	events := make(streamlog.Events, 0)
	for _, node := range nodes {
		if more, err := t.backlog(node, paths); err != nil {
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
	filters := filterMap(t.Filter)
	if events, err := streamlog.GetEventsFromObjects(paths, filters); err == nil {
		events.Render(t.Output)
	} else {
		return err
	}
	if t.Follow {
		stream, err := streamlog.GetEventStreamFromObjects(paths, filters)
		if err != nil {
			return err
		}
		for event := range stream.Events() {
			event.Render(t.Output)
		}
	}
	return nil
}

func (t *CmdObjectLogs) Run(selector, kind string) error {
	var err error
	render.SetColor(t.Color)
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "**")
	if t.Local {
		err = t.local(mergedSelector)
	} else {
		err = t.remote(mergedSelector)
	}
	return err
}
