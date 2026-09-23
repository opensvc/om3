package commoncmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdClusterRegister struct {
		CredentialFile string
		App            string
	}
)

func NewCmdClusterRegister() *cobra.Command {
	var options CmdClusterRegister
	cmd := &cobra.Command{
		Use:   "register",
		Short: "initial login on the collector, for every cluster node",
		Long: "Obtain a registration id from the collector for every cluster node, and store it in the node" +
			" configuration node.uuid keyword of that node." +
			" The collector mints a registration id for a nodename, so each node registers itself: the" +
			" collector credentials are forwarded to every node." +
			" A node already registered is registered again, with a new id.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagCollectorCredential(flags, &options.CredentialFile)
	FlagCollectorApp(flags, &options.App)
	return cmd
}

func (t *CmdClusterRegister) Run() error {
	user, password, err := CollectorCredential(t.CredentialFile)
	if err != nil {
		return err
	}
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

	body := api.PostNodeActionRegisterRequest{}
	if user != "" {
		body.User = &user
		body.Password = &password
	}
	if t.App != "" {
		body.App = &t.App
	}

	// The nodes are registered one after the other, and a node that fails
	// does not stop the ones after it: an operator asking for the cluster is
	// asking for every node, and the ones that worked are registered for
	// good.
	var errs error
	for _, node := range nodes {
		resp, err := c.PostNodeActionRegisterWithResponse(ctx, node, body)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", node, err))
			continue
		}
		switch resp.StatusCode() {
		case 204:
			fmt.Printf("%s registered\n", node)
		case 400:
			errs = errors.Join(errs, fmt.Errorf("%s: %s: %s", node, resp.JSON400.Title, resp.JSON400.Detail))
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
