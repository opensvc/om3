package commoncmd

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/opensvc/om3/core/client"
)

// PostDaemonRestart sends an api request to restart the daemon and handles the
// response status codes.
func PostDaemonRestart(ctx context.Context, cli *client.T, nodename string) error {
	_, _ = fmt.Fprintf(os.Stderr, "invoke post daemon restart on node %s\n", nodename)
	r, err := cli.PostDaemonRestart(ctx, nodename)
	if err != nil {
		return fmt.Errorf("unexpected post daemon restart failure for %s: %w", nodename, err)
	}
	switch r.StatusCode {
	case http.StatusOK:
		return nil
	default:
		return fmt.Errorf("unexpected post daemon restart status code for %s: %d", nodename, r.StatusCode)
	}
}
