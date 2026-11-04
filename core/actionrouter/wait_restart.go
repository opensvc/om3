package actionrouter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/session"
)

// restartRetryDelay is how long a daemon that answers nothing is given
// before it is asked again: it is restarting.
const restartRetryDelay = 2 * time.Second

// WaitOrchestrationThroughRestarts waits for an orchestration to end, through
// the restarts of the daemons: an upgrade of the packages restarts every
// daemon in turn, the one the client talks to among them.
//
// That daemon answers nothing while it restarts, and the wait asks again once
// it is back. Restarted, it forgot the orchestrations it knew: the one still
// running it learns of again from the monitors of its peers, and one that
// ended while it was down it never hears of, so the nodes are asked for it in
// turn, through it, when it says it does not know it. The last node to upgrade
// ends the orchestration as its daemon leaves to restart, so the end is not
// the whole of it: the wait then holds until every node is live and idle,
// which is when the upgrade is over.
//
// The wait ends when the caller's own deadline does, and a caller that set
// none waits for as long as it takes.
func WaitOrchestrationThroughRestarts(ctx context.Context, c *client.T, orchestrationID uuid.UUID, nodes []string) error {
	if orchestrationID == uuid.Nil {
		return nil
	}
	began := time.Now()
	id := orchestrationID.String()
	for {
		hold := holdDuration(ctx).String()
		resp, err := c.GetDaemonOrchestrationWithResponse(ctx, api.AliasLocalhost, id, &api.GetDaemonOrchestrationParams{Wait: &hold})
		if err != nil {
			// The daemon is restarting, or was stopped by the upgrade and
			// is not back yet.
			if err := pause(ctx, restartRetryDelay); err != nil {
				return fmt.Errorf("orchestration %s: the daemon does not answer after %s: %w", id, time.Since(began).Round(time.Second), err)
			}
			continue
		}
		switch resp.StatusCode() {
		case http.StatusOK:
			if err := orchestrationOutcome(*resp.JSON200); err != nil {
				return err
			}
			return waitNodesIdle(ctx, c, nodes)
		case http.StatusRequestTimeout:
			if hasTimeLeft(ctx) {
				continue
			}
			return fmt.Errorf("orchestration %s has not ended after %s", id, time.Since(began).Round(time.Second))
		case http.StatusServiceUnavailable, http.StatusBadGateway:
			// The daemon is up and not ready yet.
			if err := pause(ctx, restartRetryDelay); err != nil {
				return fmt.Errorf("orchestration %s: the daemon is not ready after %s: %w", id, time.Since(began).Round(time.Second), err)
			}
			continue
		case http.StatusGone:
			item, known, err := askNodesOrchestration(ctx, c, id, nodes)
			if err != nil {
				return err
			}
			if !known {
				return fmt.Errorf("no node knows orchestration %s", id)
			}
			if item.EndedAt == nil {
				// A peer still runs it, and the daemon asked will hear
				// of it again once it rejoined.
				if err := pause(ctx, restartRetryDelay); err != nil {
					return fmt.Errorf("orchestration %s has not ended after %s", id, time.Since(began).Round(time.Second))
				}
				continue
			}
			if err := orchestrationOutcome(item); err != nil {
				return err
			}
			return waitNodesIdle(ctx, c, nodes)
		default:
			return fmt.Errorf("orchestration %s: %s", id, resp.Status())
		}
	}
}

// orchestrationOutcome is how an ended orchestration went, as an error when
// it did not reach what it was for.
func orchestrationOutcome(item api.OrchestrationItem) error {
	if item.State == string(session.StateSucceeded) {
		return nil
	}
	if item.Error != nil && *item.Error != "" {
		return fmt.Errorf("orchestration %s: %s", item.State, *item.Error)
	}
	return fmt.Errorf("orchestration %s", item.State)
}

// askNodesOrchestration asks the nodes in turn, through the daemon the client
// talks to, for an orchestration that daemon does not know: the first that
// knows it answers. A node that does not answer is one restarting, and is
// passed over.
func askNodesOrchestration(ctx context.Context, c *client.T, id string, nodes []string) (api.OrchestrationItem, bool, error) {
	for _, nodename := range nodes {
		resp, err := c.GetDaemonOrchestrationWithResponse(ctx, nodename, id, &api.GetDaemonOrchestrationParams{})
		if err != nil {
			if ctx.Err() != nil {
				return api.OrchestrationItem{}, false, ctx.Err()
			}
			continue
		}
		if resp.StatusCode() == http.StatusOK && resp.JSON200 != nil {
			return *resp.JSON200, true, nil
		}
	}
	return api.OrchestrationItem{}, false, nil
}

// waitNodesIdle holds until every node is live and its monitor idle: the
// daemon of the last node to upgrade is restarting when the orchestration
// ends. A daemon not answering is asked again.
func waitNodesIdle(ctx context.Context, c *client.T, nodes []string) error {
	if len(nodes) == 0 {
		return nil
	}
	began := time.Now()
	for {
		if missing := nodesNotIdle(ctx, c, nodes); missing == "" {
			return nil
		} else if err := pause(ctx, restartRetryDelay); err != nil {
			return fmt.Errorf("%s is not idle after %s: %w", missing, time.Since(began).Round(time.Second), err)
		}
	}
}

// nodesNotIdle names the first node not live or not idle, and nothing when
// every node is.
func nodesNotIdle(ctx context.Context, c *client.T, nodes []string) string {
	resp, err := c.GetNodesWithResponse(ctx, &api.GetNodesParams{})
	if err != nil || resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		return "the daemon"
	}
	states := make(map[string]string)
	for _, item := range resp.JSON200.Items {
		if item.Data.Monitor != nil {
			states[item.Meta.Node] = item.Data.Monitor.State
		}
	}
	for _, nodename := range nodes {
		state, ok := states[nodename]
		switch {
		case !ok:
			return fmt.Sprintf("node %s is not live", nodename)
		case state != node.MonitorStateIdle.String():
			return fmt.Sprintf("node %s is %s", nodename, state)
		}
	}
	return ""
}

// pause waits for the delay, or for the context to end, which it says.
func pause(ctx context.Context, d time.Duration) error {
	if !hasTimeLeft(ctx) {
		if err := ctx.Err(); err != nil {
			return err
		}
		return errors.New("no time left")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
