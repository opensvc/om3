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
)

// maxHold is how long the daemon holds one request. A wait longer than that
// is that request asked again, which is why the daemon can cap it: what is
// waited for outlives the request, so asking again resumes the wait rather
// than restarting it.
const maxHold = time.Hour

// WaitOrchestration waits for the orchestration an action was accepted as,
// and says how it went.
//
// It asks the daemon rather than watching the event stream. The orchestration
// outlives the request in the daemon, which answers the moment it ends and
// goes on answering afterwards, so a client that asks late, or that lost its
// connection and asks again, is still told how its request went. An end event
// missed is missed for good.
//
// Any node answers for any orchestration, the monitors carrying its id
// reaching every node, so the node the action was submitted to is the one
// asked, whichever node a floating address took it to.
//
// The daemon holds one request for an hour at most, so a longer wait is that
// request asked again: the orchestration outlives it, and asking again
// resumes the wait rather than restarting it. The wait ends when the caller's
// own deadline does, and a caller that set none waits for as long as it
// takes.
func WaitOrchestration(ctx context.Context, c *client.T, orchestrationID uuid.UUID) error {
	if orchestrationID == uuid.Nil {
		// The action was refused, and the refusal is the answer. There is no
		// orchestration to wait for.
		return nil
	}
	began := time.Now()
	for {
		hold := holdDuration(ctx)
		holdS := hold.String()
		params := api.GetDaemonOrchestrationParams{Wait: &holdS}
		resp, err := c.GetDaemonOrchestrationWithResponse(ctx, api.AliasLocalhost, orchestrationID.String(), &params)
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
			return fmt.Errorf("the daemon no longer knows orchestration %s", orchestrationID)
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
// wait, capped by the longest the daemon holds one.
//
// A fifth of what is left, and a second at most, is kept for the round trip,
// so the daemon answers that the orchestration is still running rather than
// the client giving up on an answer that was on its way.
func holdDuration(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return maxHold
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
	case hold > maxHold:
		return maxHold
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
