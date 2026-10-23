package commoncmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/xsession"
)

// PostInstanceActionUnstop asks the daemon of nodename to lower the flags a
// stop raised on the instance of p, or on the resources sel selects.
func PostInstanceActionUnstop(ctx context.Context, c *client.T, p naming.Path, nodename string, sel OptsResourceSelector) (interface{}, error) {
	params := api.PostInstanceActionUnstopParams{}
	if sel.RID != "" {
		params.Rid = &sel.RID
	}
	if sel.Subset != "" {
		params.Subset = &sel.Subset
	}
	if sel.Tag != "" {
		params.Tag = &sel.Tag
	}
	{
		sessionID := xsession.SessionID().UUID()
		params.SessionID = &sessionID
	}
	response, err := c.PostInstanceActionUnstopWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name, &params)
	if err != nil {
		return nil, err
	}
	switch {
	case response.JSON200 != nil:
		return *response.JSON200, nil
	case response.JSON400 != nil:
		return nil, fmt.Errorf("%s: node %s: %s", p, nodename, *response.JSON400)
	case response.JSON401 != nil:
		return nil, fmt.Errorf("%s: node %s: %s", p, nodename, *response.JSON401)
	case response.JSON403 != nil:
		return nil, fmt.Errorf("%s: node %s: %s", p, nodename, *response.JSON403)
	case response.JSON409 != nil:
		return nil, fmt.Errorf("%s: node %s: %s", p, nodename, *response.JSON409)
	case response.JSON500 != nil:
		return nil, fmt.Errorf("%s: node %s: %s", p, nodename, *response.JSON500)
	default:
		return nil, fmt.Errorf("%s: node %s: unexpected response: %s", p, nodename, response.Status())
	}
}

// NewCmdObjectInstanceUnstop returns the instance unstop command, without its
// flags and run function, which om and ox set.
func NewCmdObjectInstanceUnstop() *cobra.Command {
	return &cobra.Command{
		Use:   "unstop",
		Short: "lower the flags a stop raised, without starting",
		Long: ForProgram(`Lower the flags a stop raised, as a start does, and start nothing.

A stop raises a flag on the instance, or on each resource it stops alone with
--rid, and the daemon does not start a flagged instance on its own, nor
restart a flagged resource. A start lowers them. Unstop lowers them and
leaves the start to the daemon: the instance is a candidate again, to take
the object over the day the instance running it fails.

The daemon may start the instance right away if the object needs it, as a
failover object down everywhere, or a flex object short of its target. A
frozen instance stays frozen: unfreeze is the operator's own decision.

With a resource selector, only the flags of the resources selected are
lowered, and the flag of the instance stays.`),
		Example: ForProgram(`  # make the instance of this node a failover candidate again
  om svc1 instance unstop

  # let the daemon restart a resource stopped alone
  om svc1 instance unstop --rid app#1

  # on another node
  om svc1 instance unstop --node n2`),
	}
}
