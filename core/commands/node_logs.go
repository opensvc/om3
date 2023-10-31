package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/streamlog"
	"github.com/opensvc/om3/util/render"
)

type (
	CmdNodeLogs struct {
		OptsGlobal
		Follow bool
		Lines  int
		Filter *[]string
	}
)

func parseFilters(l *[]string) []string {
	m := make([]string, 0)
	if l == nil {
		return m
	}
	return *l
}

func (t *CmdNodeLogs) stream(node string) {
	c, err := client.New(client.WithURL(node), client.WithTimeout(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	reader, err := c.NewGetLogs().
		SetFilters(t.Filter).
		SetLines(&t.Lines).
		SetFollow(&t.Follow).
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

func (t *CmdNodeLogs) remote() error {
	nodes, err := nodeselector.Expand(t.NodeSelector)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes to fetch logs from")
	}
	var wg sync.WaitGroup
	wg.Add(len(nodes))
	for _, node := range nodes {
		go func(n string) {
			defer wg.Done()
			t.stream(n)
		}(node)
	}
	wg.Wait()
	return nil
}

func (t *CmdNodeLogs) local() error {
	matches := parseFilters(t.Filter)
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
