package daemonapi

import (
	"context"
	"fmt"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/daemon/api"
)

// maxWait is the longest a request is held. A client that wants to wait
// longer asks again: the answer it is waiting for outlives the request, so
// asking again resumes the wait rather than restarting it, which is the
// property this has and an event stream does not.
const maxWait = time.Hour

// waitContext returns the context a request held by its wait parameter runs
// under, and whether it is held at all.
//
// Without the parameter the request is not held: the answer is what is known
// now, running or not, which is what a listing is.
func waitContext(ctx echo.Context, wait *api.Wait) (context.Context, context.CancelFunc, bool, error) {
	if wait == nil || *wait == "" {
		return ctx.Request().Context(), func() {}, false, nil
	}
	duration, err := time.ParseDuration(*wait)
	if err != nil {
		return nil, nil, false, fmt.Errorf("invalid wait duration %s: %w", *wait, err)
	}
	if duration <= 0 {
		return nil, nil, false, fmt.Errorf("invalid wait duration %s: must be positive", *wait)
	}
	if duration > maxWait {
		duration = maxWait
	}
	c, cancel := context.WithTimeout(ctx.Request().Context(), duration)
	return c, cancel, true, nil
}
