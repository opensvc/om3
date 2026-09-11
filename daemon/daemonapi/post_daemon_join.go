package daemonapi

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/converters"
)

// PostDaemonJoin orders the daemon to join the cluster of the node given in
// the body, and forks the join in the background.
//
// The requester is granted the join role, like the one adding a node to our
// own cluster nodes: both are cluster membership operations, and the token
// carrying that role is the only one holding the ca claim a foreign cluster
// needs to reach us.
//
// The join stops this daemon, moves its configuration aside and restarts it
// with the target cluster configuration, so it can not run in this handler:
// the response would never be written.
func (a *DaemonAPI) PostDaemonJoin(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRole(ctx, rbac.RoleRoot, rbac.RoleJoin); !v {
		return err
	}
	var payload api.DaemonJoinBody
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localPostDaemonJoin(ctx, payload)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename",
			"field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostDaemonJoin(ctx.Request().Context(), nodename, payload)
	})
}

func (a *DaemonAPI) localPostDaemonJoin(ctx echo.Context, payload api.DaemonJoinBody) error {
	log := LogHandler(ctx, "PostDaemonJoin")

	if payload.Node == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "missing 'node' field")
	}
	if payload.Node == a.localhost {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "field 'node' with value '%s' is localhost", payload.Node)
	}
	if payload.Token == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "missing 'token' field")
	}

	// The enroll endpoint already refused a multi node cluster, but it can
	// only see a snapshot, and this endpoint is reachable on its own. This is
	// the check that holds: our peers have no way to learn we left, so they
	// would keep us in their cluster.nodes forever.
	if nodes := clusternode.Get(); len(nodes) > 1 {
		log.Infof("join %s refused: localhost is a member of a %d nodes cluster", payload.Node, len(nodes))
		return JSONProblemf(ctx, http.StatusConflict, "Invalid state",
			"localhost is a member of a %d nodes cluster (%s): leave it first, "+
				"else its peers would keep localhost in their cluster.nodes",
			len(nodes), strings.Join(nodes, ", "))
	}

	// Fail on a token we can not use before draining anything: the ca claim is
	// the only trust anchor we have for the target cluster listener.
	if _, err := caFromToken(payload.Token); err != nil {
		log.Infof("join %s refused: %s", payload.Node, err)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body",
			"field 'token' has no usable 'ca' claim: %s (create it with the join role)", err)
	}

	timeout := enrollDefaultTimeout
	if payload.Timeout != nil && *payload.Timeout != "" {
		v, err := converters.Duration.Convert(*payload.Timeout)
		if err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body",
				"field 'timeout' with value '%s' validation error: %s", *payload.Timeout, err)
		}
		if d := *v.(*time.Duration); d > 0 {
			timeout = d
		}
	}

	execname, err := os.Executable()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "can't detect om execname: %s", err)
	}
	args := []string{"cluster", "join", "--node", payload.Node, "--timeout", timeout.String()}
	if payload.Addr != nil && *payload.Addr != "" {
		args = append(args, "--addr", *payload.Addr)
	}

	// The token goes through the environment, never through the command line:
	// /proc/<pid>/cmdline is world readable, /proc/<pid>/environ is not.
	cmd := command.New(
		command.WithName(execname),
		command.WithArgs(args),
		command.WithEnv(append(os.Environ(), env.JoinTokenVar+"="+payload.Token)),
	)

	if err := cmd.Start(); err != nil {
		log.Errorf("start cluster join: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "cluster join failed: %s", err)
	}
	log.Infof("forked a background cluster join to %s", payload.Node)
	return JSONProblemf(ctx, http.StatusOK, "background cluster join has been called", "joining the cluster of %s", payload.Node)
}
