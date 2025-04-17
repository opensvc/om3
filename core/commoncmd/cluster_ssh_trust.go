package commoncmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/spf13/cobra"
)

type (
	CmdClusterSSHTrust struct {
	}
)

func NewCmdClusterSSHTrust() *cobra.Command {
	var options CmdClusterSSHTrust
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "ssh-trust all the node mesh",
		Long: "Configure all nodes to allow SSH communication from their peers." +
			" By default, the trusted SSH key is opensvc, but this can be customized using the node.sshkey setting." +
			" If the key does not exist, OpenSVC automatically generates it.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	return cmd
}

func (t *CmdClusterSSHTrust) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	nodes, err := nodeselector.New("*", nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found")
	}
	ctx := context.Background()

	var errs error
	for _, node := range nodes {
		resp, err := c.PutNodeSSHTrustWithResponse(ctx, node)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", node, err))
			continue
		}
		switch resp.StatusCode() {
		case 204:
		case 401:
			errs = errors.Join(errs, fmt.Errorf("%s: %s: %s", node, resp.JSON401.Title, resp.JSON401.Detail))
		case 403:
			errs = errors.Join(errs, fmt.Errorf("%s: %s: %s", node, resp.JSON403.Title, resp.JSON403.Detail))
		case 500:
			errs = errors.Join(errs, fmt.Errorf("%s: %s: %s", node, resp.JSON500.Title, resp.JSON500.Detail))
		default:
			errs = errors.Join(errs, fmt.Errorf("%s: unexpected status: %s", node, resp.Status()))
		}
	}
	return errs
}
