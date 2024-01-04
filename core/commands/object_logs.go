package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/streamlog"
	"github.com/opensvc/om3/util/render"
	"github.com/opensvc/om3/util/xmap"
)

type (
	CmdObjectLogs struct {
		OptsGlobal
		OptsLogs
		NodeSelector string
	}
)

/*
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
*/

func (t *CmdObjectLogs) stream(node string, paths naming.Paths) {
	c, err := client.New(client.WithURL(node), client.WithTimeout(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	l := paths.StrSlice()
	reader, err := c.NewGetLogs().
		SetFilters(&t.Filter).
		SetLines(&t.Lines).
		SetFollow(&t.Follow).
		SetPaths(&l).
		GetReader()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer reader.Close()

	for {
		event, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
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
	c, err := client.New(client.WithURL(t.Server), client.WithTimeout(0))
	if err != nil {
		return err
	}
	sel := objectselector.NewSelection(
		selStr,
		objectselector.SelectionWithLocal(true),
	)
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	if t.NodeSelector != "" {
		nodes, err = nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
		if err != nil {
			return err
		}
	} else {
		nodes, err = nodesFromPaths(paths)
		if err != nil {
			return err
		}
	}
	var wg sync.WaitGroup
	wg.Add(len(nodes))
	for _, node := range nodes {
		go func(n string) {
			defer wg.Done()
			t.stream(n, paths)
		}(node)
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
	matches := parseFilters(&t.Filter)
	last := len(paths) - 1
	for i, path := range paths {
		matches = append(matches, "OBJ_PATH="+path.String())
		if i > 0 && i < last {
			matches = append(matches, "+")
		}
	}
	stream := streamlog.NewStream()
	streamConfig := streamlog.StreamConfig{
		Follow:  t.Follow,
		Lines:   t.Lines,
		Matches: matches,
	}
	if err := stream.Start(streamConfig); err != nil {
		return err
	}
	defer stream.Stop()
	for {
		select {
		case err := <-stream.Errors():
			fmt.Fprintln(os.Stderr, err)
			if err == nil {
				// The sender has stopped sending
				return nil
			}
		case ev := <-stream.Events():
			ev.Render(t.Output)
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
