package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
)

// NetworkSetup asks the node monitor of each node of nodenames to set up the
// networks names, all of them when names is empty, and waits for the setups
// to run. The node monitor runs one setup at a time, so a setup asked never
// writes the firewall rules of the networks along with another.
func NetworkSetup(ctx context.Context, c *client.T, nodenames, names []string) error {
	params := api.PostNodeNetworkSetupParams{}
	if len(names) > 0 {
		params.Name = &names
	}
	var errs error
	for _, nodename := range nodenames {
		resp, err := c.PostNodeNetworkSetupWithResponse(ctx, nodename, &params)
		switch {
		case err != nil:
			errs = errors.Join(errs, fmt.Errorf("%s: %w", nodename, err))
		case resp.StatusCode() == http.StatusNoContent:
		default:
			errs = errors.Join(errs, fmt.Errorf("%s: %s", nodename, networkSetupProblem(resp)))
		}
	}
	return errs
}

func networkSetupProblem(resp *api.PostNodeNetworkSetupResponse) string {
	switch {
	case resp.JSON400 != nil:
		return resp.JSON400.Detail
	case resp.JSON401 != nil:
		return resp.JSON401.Detail
	case resp.JSON403 != nil:
		return resp.JSON403.Detail
	case resp.JSON408 != nil:
		return resp.JSON408.Detail
	case resp.JSON500 != nil:
		return resp.JSON500.Detail
	}
	return fmt.Sprintf("unexpected status %s", resp.Status())
}
