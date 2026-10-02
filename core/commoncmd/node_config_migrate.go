package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdNodeConfigMigrate struct {
		NodeSelector string
		DryRun       bool
	}
)

// Run writes the configurations of the selected nodes in the shape om reads
// them in, as CmdObjectConfigMigrate does for objects.
func (t *CmdNodeConfigMigrate) Run() error {
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
	var errs error
	for _, nodename := range nodenames {
		errs = errors.Join(errs, t.migrate(c, nodename, len(nodenames) > 1))
	}
	return errs
}

func (t *CmdNodeConfigMigrate) migrate(c *client.T, nodename string, prefixed bool) error {
	prefix := ""
	if prefixed {
		prefix = nodename + ": "
	}
	b, err := nodeConfigFile(c, nodename)
	if err != nil {
		return err
	}
	n, err := object.NewNode(object.WithConfigData(b), object.WithVolatile(true))
	if err != nil {
		return fmt.Errorf("%s: %w", nodename, err)
	}
	m := object.MigrateNodeConfig(n.Config())
	sets, unsets, deletes, ok := showMigration(prefix, m)
	if !ok || t.DryRun {
		return nil
	}
	backup, err := backupConfig("node.conf."+nodename, b)
	if err != nil {
		// A change nothing can undo is not one to make: the configuration
		// this rewrites is the only copy of what the node was.
		return fmt.Errorf("%s: keep a copy of the configuration before changing it: %w", nodename, err)
	}
	fmt.Printf("%sthe configuration as it was is kept in %s\n", prefix, backup)
	params := api.PatchNodeConfigParams{
		Set:    &sets,
		Unset:  &unsets,
		Delete: &deletes,
	}
	response, err := c.PatchNodeConfigWithResponse(context.Background(), nodename, &params)
	if err != nil {
		return err
	}
	switch response.StatusCode() {
	case 200:
		if response.JSON200.IsChanged {
			fmt.Printf("%scommitted\n", prefix)
		} else {
			fmt.Printf("%sunchanged\n", prefix)
		}
	case 400:
		return fmt.Errorf("%s: %s", nodename, *response.JSON400)
	case 401:
		return fmt.Errorf("%s: %s", nodename, *response.JSON401)
	case 403:
		return fmt.Errorf("%s: %s", nodename, *response.JSON403)
	case 500:
		return fmt.Errorf("%s: %s", nodename, *response.JSON500)
	default:
		return fmt.Errorf("%s: unexpected response: %s", nodename, response.Status())
	}
	return nil
}

// nodeConfigFile is the configuration of a node, as the node has it.
func nodeConfigFile(c *client.T, nodename string) ([]byte, error) {
	params := api.GetNodeConfigFileParams{}
	resp, err := c.GetNodeConfigFileWithResponse(context.Background(), nodename, &params)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", nodename, err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("%s: read the configuration: %s: %s", nodename, resp.Status(), strings.TrimSpace(string(resp.Body)))
	}
	return resp.Body, nil
}
