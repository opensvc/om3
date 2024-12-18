package oxcmd

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
	CmdClusterLogs struct {
		OptsGlobal
		OptsLogs
		NodeSelector string
	}
)

func (t *CmdClusterLogs) stream(node string) {
	c, err := client.New(client.WithURL(node), client.WithTimeout(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	reader, err := c.NewGetLogs(node).
		SetFilters(&t.Filter).
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

func (t *CmdClusterLogs) remote() error {
	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}
	nodes, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
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

func (t *CmdClusterLogs) Run() error {
	render.SetColor(t.Color)
	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}
	return t.remote()
}
