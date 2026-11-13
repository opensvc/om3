package commoncmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/xsession"
)

// ObjectKeyInstallRemote returns the remote function of "<path> key
// install": the keys of the datastore, the key named or all of them, are
// installed again in the volumes of the node projecting them, by an exec the
// node starts.
func ObjectKeyInstallRemote(name string) func(context.Context, naming.Path, string) (any, error) {
	return func(ctx context.Context, p naming.Path, nodename string) (any, error) {
		c, err := client.New()
		if err != nil {
			return nil, err
		}
		return objectKeyInstall(ctx, c, p, nodename, name)
	}
}

func objectKeyInstall(ctx context.Context, c *client.T, p naming.Path, nodename, name string) (any, error) {
	sessionID := xsession.SessionID().UUID()
	params := api.PostInstanceActionKeyInstallParams{Name: ptrOrNil(name), SessionID: &sessionID}
	resp, err := c.PostInstanceActionKeyInstallWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name, &params)
	if err != nil {
		return nil, fmt.Errorf("%s: node %s: %w", p, nodename, err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("%s: %w", p, nodeResponseError(nodename, resp, resp.Body))
	}
	return *resp.JSON200, nil
}
