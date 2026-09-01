package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdNodeMetrics struct {
		NodeSelector string
	}
)

func NewCmdNodeMetrics() *cobra.Command {
	options := CmdNodeMetrics{}
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "print the prometheus metrics of the selected nodes",
		Long: "Print what the node listener exposes at /metrics, which is what a\n" +
			"prometheus scrape of this cluster collects.\n\n" +
			"The metrics of the high cardinality subsystems are served apart, at\n" +
			"/metrics/<subsystem>, and are not printed here.\n\n" +
			"A user without the root grant is served the metrics of the objects of\n" +
			"the namespaces they have a grant on, and none of the daemon's own.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	FlagNodeSelector(cmd.Flags(), &options.NodeSelector)
	return cmd
}

func (t *CmdNodeMetrics) Run() error {
	if t.NodeSelector == "" {
		if clientcontext.IsSet() {
			return fmt.Errorf("--node must be set")
		}
		t.NodeSelector = hostname.Hostname()
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if errors.Is(err, nodeselector.ErrClusterNodeCacheEmpty) {
		nodenames = []string{hostname.Hostname()}
	} else if err != nil {
		return err
	}
	if len(nodenames) == 0 {
		return fmt.Errorf("no node to read metrics from")
	}
	var errs error
	for _, nodename := range nodenames {
		if len(nodenames) > 1 {
			// A comment, so that the concatenation of several nodes
			// stays readable by what reads an exposition format.
			fmt.Printf("# node %s\n", nodename)
		}
		if err := t.doNode(c, nodename); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func (t *CmdNodeMetrics) doNode(c *client.T, nodename string) error {
	resp, err := c.GetNodeMetrics(context.Background(), nodename)
	if err != nil {
		return fmt.Errorf("get metrics from node %s: %w", nodename, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxSubActionBodySize))
		if s := problemString(b); s != "" {
			return fmt.Errorf("get metrics from node %s: %s", nodename, s)
		}
		return fmt.Errorf("get metrics from node %s: unexpected status code %d", nodename, resp.StatusCode)
	}
	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		return fmt.Errorf("read metrics from node %s: %w", nodename, err)
	}
	return nil
}
