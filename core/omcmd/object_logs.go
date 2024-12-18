package omcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/streamlog"
	"github.com/opensvc/om3/daemon/api"
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

func (t *CmdObjectLogs) stream(c *client.T, node string, paths naming.Paths) {
	l := paths.StrSlice()
	reader, err := c.NewGetLogs(node).
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

func nodesFromPaths(c *client.T, selector string) ([]string, error) {
	m := make(map[string]any)
	params := api.GetObjectsParams{Path: &selector}
	resp, err := c.GetObjectsWithResponse(context.Background(), &params)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("%s", resp.Status())
	}
	for _, item := range resp.JSON200.Items {
		for node := range item.Data.Instances {
			m[node] = nil
		}
	}
	return xmap.Keys(m), nil
}

func (t *CmdObjectLogs) remote(selStr string) error {
	var (
		paths naming.Paths
		nodes []string
		err   error
	)
	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}
	if paths, err = objectselector.New(selStr, objectselector.WithClient(c)).Expand(); err != nil {
		return err
	}
	if t.NodeSelector != "" {
		nodes, err = nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
		if err != nil {
			return err
		}
	} else {
		nodes, err = nodesFromPaths(c, selStr)
		if err != nil {
			return err
		}
	}
	var wg sync.WaitGroup
	wg.Add(len(nodes))
	for _, node := range nodes {
		go func(n string) {
			defer wg.Done()
			t.stream(c, n, paths)
		}(node)
	}
	wg.Wait()
	return nil
}

func (t *CmdObjectLogs) local(selStr string) error {
	sel := objectselector.New(
		selStr,
		objectselector.WithLocal(true),
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
			if err == nil {
				// The sender has stopped sending
				return nil
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
		case ev := <-stream.Events():
			ev.Render(t.Output)
		}
	}
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
