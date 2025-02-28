package commoncmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

var (
	DaemonComponentAllowedActions = []string{
		string(api.InPathDaemonComponentActionStart),
		string(api.InPathDaemonComponentActionStop),
		string(api.InPathDaemonComponentActionRestart),
	}
)

// PostDaemonComponentAction performs an action on specific daemon
// subcomponents for a given node.
// It sends a POST request to execute the provided action on the specified
// subcomponents on the target node.
func PostDaemonComponentAction(ctx context.Context, cli *client.T, nodename string, action string, sub []string) error {
	subs := strings.Join(sub, ", ")
	_, _ = fmt.Fprintf(os.Stderr, "Invoke action %s on node %s daemon components %s\n",
		action, nodename, subs)
	body := api.PostDaemonComponentActionJSONRequestBody{
		Subs: sub,
	}
	actionParam := api.PostDaemonComponentActionParamsAction(action)
	r, err := cli.PostDaemonComponentAction(ctx, nodename, actionParam, body)
	if err != nil {
		return fmt.Errorf("Invoke action %s on node %s daemon components %s: %w",
			action, nodename, subs, err)
	}
	switch r.StatusCode {
	case http.StatusOK:
		return nil
	default:
		return fmt.Errorf("Invoke action %s on node %s daemon sub-components %s: unexpected status code %d",
			action, nodename, subs, r.StatusCode)
	}
}
