package oxcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/resourceid"
)

type (
	CmdObjectContainerLogs struct {
		OptsGlobal
		NodeSelector string
		RID          string
		Follow       bool
		Lines        int
	}
)

func (t *CmdObjectContainerLogs) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "**")

	if t.RID == "" {
		return fmt.Errorf("rid is required")
	}

	rid, err := resourceid.Parse(t.RID)
	if err != nil {
		return fmt.Errorf("invalid rid: %w", err)
	}

	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}

	paths, err := objectselector.New(mergedSelector, objectselector.WithClient(c)).MustExpand()
	if err != nil {
		return err
	}

	if len(paths) > 1 {
		return fmt.Errorf("only one path is accepted: selected %s", paths)
	}

	var nodes []string
	if t.NodeSelector != "" {
		nodes, err = nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
		if err != nil {
			return err
		}
	} else {
		nodes, err = commoncmd.NodesFromPaths(c, mergedSelector)
		if err != nil {
			return err
		}
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no nodes selected")
	}

	// For container logs, we typically want logs from the node where the container is running
	// So we'll use the first node
	node := nodes[0]
	path := paths[0]

	// Get the container logs stream
	containerLogs := c.NewGetContainerLogs(path, node, rid.String())
	logChan, err := containerLogs.Logs(context.Background(), t.Follow, t.Lines)
	if err != nil {
		return fmt.Errorf("failed to get container logs: %w", err)
	}

	// Stream the logs to stdout
	for logData := range logChan {
		_, err := os.Stdout.Write(logData)
		if err != nil {
			return fmt.Errorf("failed to write log output: %w", err)
		}
	}

	return nil
}
