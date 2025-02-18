package commoncmd

import (
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
)

type (
	// CmdObjectLogs duplicates the omcmd and oxcmd CmdObjectLogs configuration
	// for fetching logs from nodes based on the specified NodeSelector.
	CmdObjectLogs struct {
		OptsGlobal
		OptsLogs
		NodeSelector string
	}
)

// Remote executes the object log retrieval operation for selected nodes and
// paths in an asynchronous manner.
// It leverages the provided selector string to identify target objects and
// nodes for log streaming.
func (t *CmdObjectLogs) Remote(selStr string) error {
	var (
		paths naming.Paths
		nodes []string
		err   error
	)
	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}
	if paths, err = objectselector.New(selStr, objectselector.WithClient(c)).MustExpand(); err != nil {
		return err
	}
	if t.NodeSelector != "" {
		nodes, err = nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
		if err != nil {
			return err
		}
	} else {
		nodes, err = NodesFromPaths(c, selStr)
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
