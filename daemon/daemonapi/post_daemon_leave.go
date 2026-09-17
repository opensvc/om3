package daemonapi

import (
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/credential"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/converters"
)

// PostDaemonLeave orders the daemon to leave its cluster, and forks the leave
// in the background.
//
// The requester is granted the leave role, like the one dropping a node from
// our own cluster nodes: both are cluster membership operations.
//
// The leave stops this daemon, moves its configuration aside and restarts it
// alone, so it cannot run in this handler: the response would never be
// written.
func (a *DaemonAPI) PostDaemonLeave(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRole(ctx, rbac.RoleRoot, rbac.RoleLeave); !v {
		return err
	}
	var payload api.DaemonLeaveBody
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localPostDaemonLeave(ctx, payload)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename",
			"field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostDaemonLeave(ctx.Request().Context(), nodename, payload)
	})
}

func (a *DaemonAPI) localPostDaemonLeave(ctx echo.Context, payload api.DaemonLeaveBody) error {
	log := LogHandler(ctx, "PostDaemonLeave")

	// Fail on a credential we cannot use before draining anything: it is the
	// only way left to reach our api once we are alone, with a cluster secret
	// of our own that none of the credentials of the cluster we leave match.
	cred := ""
	if payload.Credential != nil && *payload.Credential != "" {
		if _, _, err := credential.Parse(*payload.Credential); err != nil {
			log.Infof("leave refused: %s", err)
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "field 'credential': %s", err)
		}
		cred = *payload.Credential
	}

	// The evict endpoint already refused a single node cluster, but it can
	// only see a snapshot, and this endpoint is reachable on its own. The
	// leave asks a peer to drop us from the cluster nodes: alone, there is no
	// peer to ask, and nothing to leave.
	if nodes := clusternode.Get(); len(nodes) < 2 {
		log.Infof("leave refused: localhost is a single node cluster")
		return JSONProblemf(ctx, http.StatusConflict, "Invalid state",
			"localhost is a single node cluster: there is no peer to be removed by")
	}

	timeout := evictDefaultTimeout
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
	args := []string{"cluster", "leave", "--timeout", timeout.String()}

	// The credential goes through the environment, never through the command
	// line: /proc/<pid>/cmdline is world readable, /proc/<pid>/environ is not.
	environ := os.Environ()
	if cred != "" {
		environ = append(environ, env.CredentialVar+"="+cred)
	}
	cmd := command.New(
		command.WithName(execname),
		command.WithArgs(args),
		command.WithEnv(environ),
	)

	if err := cmd.Start(); err != nil {
		log.Errorf("start cluster leave: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "cluster leave failed: %s", err)
	}
	log.Infof("forked a background cluster leave")
	return JSONProblemf(ctx, http.StatusOK, "background cluster leave has been called", "leaving the cluster")
}
