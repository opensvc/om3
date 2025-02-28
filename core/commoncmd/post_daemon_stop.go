package commoncmd

import (
	"context"
	"fmt"
	"os"

	"github.com/opensvc/om3/core/client"
)

// PostDaemonStop sends an api request to stop the daemon and handles the
// response status codes.
func PostDaemonStop(ctx context.Context, cli *client.T, nodename string) error {
	r, err := cli.PostDaemonStopWithResponse(ctx, nodename)
	if err != nil {
		return fmt.Errorf("unexpected post daemon stop failure for %s: %w", nodename, err)
	}
	switch {
	case r.JSON200 != nil:
		_, _ = fmt.Fprintf(os.Stderr, "stopping daemon on node %s with pid %d\n", nodename, r.JSON200.Pid)
		return nil
	default:
		return fmt.Errorf("unexpected post daemon stop status code for %s: %d", nodename, r.StatusCode)
	}
}
