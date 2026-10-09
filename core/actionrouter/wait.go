package actionrouter

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/session"
	"github.com/opensvc/om3/v3/util/hostname"
)

// maxHold is how long the daemon holds one request. A wait longer than that
// is that request asked again, which is why the daemon can cap it: what is
// waited for outlives the request, so asking again resumes the wait rather
// than restarting it.
const maxHold = time.Hour

// maxForwardedHold is how long a request the daemon forwards to a peer is
// held. The daemon forwards it with a client that gives up on an answer after
// client.DefaultClientTimeout, so the peer has to answer before that: asking
// again resumes the wait.
var maxForwardedHold = client.DefaultClientTimeout * 2 / 3

// WaitOrchestration waits for the orchestration an action was accepted as,
// and says how it went.
//
// It asks the daemon rather than watching the event stream. The orchestration
// outlives the request in the daemon, which answers the moment it ends and
// goes on answering afterwards, so a client that asks late, or that lost its
// connection and asks again, is still told how its request went. An end event
// missed is missed for good.
//
// The daemon the client talks to is asked: it is the node an object action
// was submitted to, which accepted it and records it before answering its id,
// whichever node a floating address took it to.
//
// The daemon holds one request for an hour at most, so a longer wait is that
// request asked again: the orchestration outlives it, and asking again
// resumes the wait rather than restarting it. The wait ends when the caller's
// own deadline does, and a caller that set none waits for as long as it
// takes.
func WaitOrchestration(ctx context.Context, c *client.T, orchestrationID uuid.UUID) error {
	return waitOrchestration(ctx, c, api.AliasLocalhost, orchestrationID)
}

// WaitNodeOrchestration waits for the orchestration a node action was
// accepted as by nodename, and says how it went, as WaitOrchestration does.
//
// nodename is asked first: it accepted the orchestration and recorded it
// before answering its id, so it knows the id at once, and knows how it
// ended, a refusal included. Another node knows the id only once the node
// monitor carrying it reached it over the heartbeats, and answers an id it
// does not know yet as one it dropped.
//
// The daemon the client talks to is asked next, until it too has seen the
// end: its copy of the node monitor of nodename is what the next request
// is judged on, as an evict following a drain. Its answer says nothing more
// of how it went, and an id it never saw, of an orchestration that ended
// before the heartbeats told it, is no error.
func WaitNodeOrchestration(ctx context.Context, c *client.T, nodename string, orchestrationID uuid.UUID) error {
	if nodename == "" || nodename == api.AliasLocalhost || nodename == hostname.Hostname() {
		return WaitOrchestration(ctx, c, orchestrationID)
	}
	if err := waitOrchestration(ctx, c, nodename, orchestrationID); err != nil {
		return err
	}
	_ = waitOrchestration(ctx, c, api.AliasLocalhost, orchestrationID)
	return nil
}

func waitOrchestration(ctx context.Context, c *client.T, nodename string, orchestrationID uuid.UUID) error {
	if orchestrationID == uuid.Nil {
		// The action was refused, and the refusal is the answer. There is no
		// orchestration to wait for.
		return nil
	}
	capHold := maxHold
	if nodename != api.AliasLocalhost {
		capHold = maxForwardedHold
	}
	began := time.Now()
	for {
		hold := holdDuration(ctx, capHold)
		holdS := hold.String()
		params := api.GetDaemonOrchestrationParams{Wait: &holdS}
		resp, err := c.GetDaemonOrchestrationWithResponse(ctx, nodename, orchestrationID.String(), &params)
		if err != nil {
			return err
		}
		switch resp.StatusCode() {
		case http.StatusOK:
		case http.StatusRequestTimeout:
			if hasTimeLeft(ctx) {
				// The hold expired, not the wait: ask again.
				continue
			}
			return fmt.Errorf("orchestration %s has not ended after %s", orchestrationID, time.Since(began).Round(time.Second))
		case http.StatusGone:
			return fmt.Errorf("the daemon of %s no longer knows orchestration %s", nodename, orchestrationID)
		default:
			return fmt.Errorf("orchestration %s: %s", orchestrationID, resp.Status())
		}
		item := *resp.JSON200
		if item.State == string(session.StateSucceeded) {
			return nil
		}
		if item.Error != nil && *item.Error != "" {
			return fmt.Errorf("orchestration %s: %s", item.State, *item.Error)
		}
		return fmt.Errorf("orchestration %s", item.State)
	}
}

// holdDuration is how long the next request is held: what is left of the
// wait, capped by capHold, the longest the request can be held.
//
// A fifth of what is left, and a second at most, is kept for the round trip,
// so the daemon answers that the orchestration is still running rather than
// the client giving up on an answer that was on its way.
func holdDuration(ctx context.Context, capHold time.Duration) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return capHold
	}
	remaining := time.Until(deadline)
	grace := time.Second
	if fifth := remaining / 5; fifth < grace {
		grace = fifth
	}
	hold := remaining - grace
	switch {
	case hold <= 0:
		return time.Millisecond
	case hold > capHold:
		return capHold
	default:
		return hold
	}
}

// hasTimeLeft says the caller is still waiting: it set no deadline, or its
// deadline is far enough away for another request to be worth making.
func hasTimeLeft(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return true
	}
	return time.Until(deadline) > 100*time.Millisecond
}
