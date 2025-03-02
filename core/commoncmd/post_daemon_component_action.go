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
		string(api.DaemonSubsystemActionStart),
		string(api.DaemonSubsystemActionStop),
		string(api.DaemonSubsystemActionRestart),
	}
)

// PostDaemonComponentAction performs an action on specific daemon
// subcomponents for a given node.
// It sends a POST request to execute the provided action on the specified
// subcomponents on the target node.
func PostDaemonComponentAction(ctx context.Context, sub string, cli *client.T, nodename string, action string, name []string) error {
	names := strings.Join(name, ", ")
	_, _ = fmt.Fprintf(os.Stderr, "Invoke on node %s: action daemon %s %s for %s\n",
		nodename, sub, action, names)
	body := api.DaemonSubNameBody{
		Name: name,
	}
	poster, err := cli.NewPostDaemonSubFunc(sub)
	if err != nil {
		return err
	}
	actionParam := api.InPathDaemonSubAction(action)
	r, err := poster(ctx, nodename, actionParam, body)
	if err != nil {
		return fmt.Errorf("Invoked on node %s: action daemon %s %s for %s: %w",
			nodename, sub, action, names, err)
	}
	switch r.StatusCode {
	case http.StatusOK:
		return nil
	default:
		return fmt.Errorf("Invoked on node %s: action daemon %s %s for %s: unexpected status code %d",
			nodename, sub, action, names, r.StatusCode)
	}
}
