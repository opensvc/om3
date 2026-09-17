package daemonapi

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/credential"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/converters"
)

// evictDefaultTimeout is the duration the leave forked on the evicted node
// runs under when the body sets no timeout. It has to outlive a daemon
// restart.
const evictDefaultTimeout = time.Hour

// PostClusterEvict removes a node from our cluster nodes.
//
// The node to evict is ordered to leave: it is the one that asks a peer to
// drop it from the cluster nodes, then restarts alone. This handler is the
// entry point an operator uses from any other node of the cluster.
//
// The node to evict must be drained. Nothing in the leave flow stops what it
// still runs, so evicting a node with live instances would leave them running
// on a node the cluster no longer knows about, free to start a second time
// elsewhere.
func (a *DaemonAPI) PostClusterEvict(ctx echo.Context) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	log := LogHandler(ctx, "PostClusterEvict")

	var payload api.ClusterEvictBody
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	if payload.Nodename == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "missing 'nodename' field")
	}
	nodename := a.parseNodename(payload.Nodename)

	// Evicting ourselves would mean ordering ourselves to leave, and the leave
	// needs a peer to be removed by. Posting this to that peer is the same
	// operation, seen from a node that survives it.
	if nodename == a.localhost {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body",
			"field 'nodename' with value '%s' is localhost: post this request to one of its peers", nodename)
	}
	if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body",
			"field 'nodename' with value '%s' is not a cluster node", nodename)
	}

	// Fail on a credential we can not use before ordering anything: the
	// evicted node would find out on its own, after the point where it is no
	// longer reachable with the credentials of this cluster.
	if payload.Credential != nil && *payload.Credential != "" {
		if _, _, err := credential.Parse(*payload.Credential); err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "field 'credential': %s", err)
		}
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

	if reason := notDrainedReason(nodename); reason != "" {
		log.Infof("evict %s refused: %s", nodename, reason)
		return JSONProblemf(ctx, http.StatusConflict, "Invalid state",
			"node '%s' is not drained: %s: drain it first, else what it runs would stay up on a node "+
				"the cluster no longer knows about", nodename, reason)
	}

	timeoutStr := timeout.String()
	leaveBody := api.PostDaemonLeaveJSONRequestBody{
		Timeout: &timeoutStr,
	}
	if payload.Credential != nil && *payload.Credential != "" {
		leaveBody.Credential = payload.Credential
	}

	cli, err := a.newProxyClient(ctx, nodename)
	if err != nil {
		log.Errorf("create proxy client for %s: %s", nodename, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}

	log.Infof("order node %s to leave the cluster", nodename)
	resp, err := cli.PostDaemonLeaveWithResponse(ctx.Request().Context(), nodename, leaveBody)
	if err != nil {
		log.Warnf("post daemon leave on %s: %s", nodename, err)
		return JSONProblemf(ctx, http.StatusBadGateway, "Evicted node",
			"post daemon leave on '%s': %s", nodename, err)
	}
	if code := resp.StatusCode(); code != http.StatusOK {
		detail := strings.TrimSpace(string(resp.Body))
		log.Warnf("post daemon leave on %s: got %d: %s", nodename, code, detail)
		// Relay a refusal from the evicted node under its own status: it
		// re-checks what we checked, and a 409 it raises between our check and
		// this post is the same conflict, not a gateway error.
		if code >= 400 && code < 500 {
			return JSONProblemf(ctx, code, "Evicted node",
				"node '%s' refused the leave order: %s", nodename, detail)
		}
		return JSONProblemf(ctx, http.StatusBadGateway, "Evicted node",
			"post daemon leave on '%s': got %d wanted %d: %s", nodename, code, http.StatusOK, detail)
	}
	log.Infof("node %s accepted the leave order", nodename)
	return JSONProblemf(ctx, http.StatusOK, "background cluster leave has been called",
		"node %s is leaving the cluster", nodename)
}

// notDrainedReason returns why nodename is not drained, and an empty string
// when it is.
//
// A drain leaves no mark of its own: nmon resets the node monitor to idle as
// soon as it succeeds, so what a drained node is recognized by is its result,
// a frozen node running nothing, not a state the drain sets.
func notDrainedReason(nodename string) string {
	nodeStatus := node.StatusData.GetByNode(nodename)
	if nodeStatus == nil {
		return "its status is unknown to this node"
	}
	if !nodeStatus.IsFrozen() {
		return "it is not frozen"
	}

	// A drain in progress ends frozen too, so a snapshot of the instances is
	// not enough to tell a drained node from one still stopping.
	if mon := node.MonitorData.GetByNode(nodename); mon != nil {
		switch {
		case mon.LocalExpect == node.MonitorLocalExpectDrained:
			return "a drain is in progress"
		case mon.State == node.MonitorStateDrainProgress:
			return "a drain is in progress"
		case mon.State == node.MonitorStateDrainFailure:
			return "its last drain failed"
		}
	}

	up := make([]string, 0)
	for p, instanceStatus := range instance.StatusData.GetByNode(nodename) {
		if p.Kind != naming.KindSvc {
			continue
		}
		switch instanceStatus.Avail {
		case status.Up, status.Warn, status.StandbyUpWithUp:
			up = append(up, p.String())
		}
	}
	if len(up) > 0 {
		sort.Strings(up)
		return fmt.Sprintf("it still runs %s", strings.Join(up, ", "))
	}
	return ""
}
