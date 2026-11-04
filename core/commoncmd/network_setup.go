package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/opensvc/om3/v3/core/actionlogs"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/xsession"
)

// NetworkSetupClientTimeout is the timeout of the client asking a network
// setup: the api answers once the setup ran, waiting up to 2 minutes for it
// (networkSetupTimeout of the daemon api), longer than the default client
// timeout. A client giving up first would say failed a setup still running.
const NetworkSetupClientTimeout = 2*time.Minute + 15*time.Second

// NetworkSetup asks the node monitor of each node of nodenames to set up the
// networks names, all of them when names is empty, and waits for the setups
// to run, on every node at once. The node monitor runs one setup at a time,
// so a setup asked never writes the firewall rules of the networks along
// with another.
//
// The setups log with the session id of this command, and their logs are
// streamed in format as they run, the setup running in the daemons.
func NetworkSetup(ctx context.Context, c *client.T, nodenames, names []string, format string) error {
	sessionID := xsession.SessionID().UUID()
	params := api.PostNodeNetworkSetupParams{SessionID: &sessionID}
	if len(names) > 0 {
		params.Name = &names
	}
	// Opened before the setups are asked, so the stream has their first
	// entries.
	logs, err := actionlogs.StartSessionLogs(c, nodenames, sessionID, format)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	defer logs.Stop()
	// The nodes are asked at once: each answers within the 2 minutes the
	// api waits for its setup, so the log streams, bounded by the client
	// timeout from their opening, see every setup to its end.
	type result struct {
		nodename string
		err      error
	}
	results := make(chan result, len(nodenames))
	for _, nodename := range nodenames {
		go func(nodename string) {
			resp, err := c.PostNodeNetworkSetupWithResponse(ctx, nodename, &params)
			switch {
			case err != nil:
				err = fmt.Errorf("%s: %w", nodename, err)
			case resp.StatusCode() == http.StatusNoContent:
			default:
				err = fmt.Errorf("%s: %s", nodename, networkSetupProblem(resp))
			}
			results <- result{nodename: nodename, err: err}
		}(nodename)
	}
	errsByNode := make(map[string]error, len(nodenames))
	for range nodenames {
		r := <-results
		errsByNode[r.nodename] = r.err
	}
	var errs error
	for _, nodename := range nodenames {
		errs = errors.Join(errs, errsByNode[nodename])
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
