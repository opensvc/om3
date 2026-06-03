package commoncmd

import (
	"fmt"
	"os"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/util/logreader"
)

type (
	// CmdNodeLogs duplicates the omcmd and oxcmd CmdNodeLogs configuration for
	// fetching logs from nodes based on the specified NodeSelector.
	CmdNodeLogs struct {
		OptsGlobal
		OptsLogs
		NodeSelector string
	}
)

// Remote fetches logs remotely from the nodes specified by the NodeSelector,
// using concurrent streaming for each node.
// Returns an error if the client setup fails, no nodes are selected,
// or if there are issues during the process.
func (t *CmdNodeLogs) Remote() error {
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
	
	// Create readers for all nodes
	streams := make([]logreader.NodeStream, 0, len(nodes))
	for _, node := range nodes {
		reader, err := c.NewGetLogs(node).
			SetFilters(&t.Filter).
			SetGrep(t.Grep).
			SetLines(&t.Lines).
			SetFollow(&t.Follow).
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
	// Pass os.Stdout as the writer (the actual output destination)
	logreader.CollectAndSortWithFormat(
		streams,
		os.Stdout,  // output writer
		t.Output,   // format (e.g., "", "json")
		t.Follow,  // follow mode
	)
	
	return nil
}
