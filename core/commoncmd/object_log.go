package commoncmd

import (
	"fmt"
	"os"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/util/logreader"
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

	// Create readers for all nodes
	streams := make([]logreader.NodeStream, 0, len(nodes))
	l := paths.StrSlice()
	for _, node := range nodes {
		reader, err := c.NewGetLogs(node).
			SetFilters(&t.Filter).
			SetGrep(t.Grep).
			SetLines(&t.Lines).
			SetFollow(&t.Follow).
			SetPaths(&l).
			GetReader()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		streams = append(streams, logreader.NodeStream{
			Node:   node,
			Reader: reader,
		})
	}

	if len(streams) == 0 {
		return fmt.Errorf("no valid log streams to read from")
	}

	// Use the logreader utility to collect, sort, and display logs
	logreader.CollectAndSortWithFormat(
		streams,
		os.Stdout, // output writer
		t.Output,  // format (e.g., "", "json")
		t.Follow,  // follow mode
	)

	return nil
}
