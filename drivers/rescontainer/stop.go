package rescontainer

import (
	"context"
	"time"
)

// StopKillMargin is what a stop leaves, of the time of its action, to the
// forced stop of a container its shutdown did not stop, as the lxc driver
// leaves its kill: the wait for the shutdown ending with the deadline of the
// action, the forced stop ran out of time, and the container stayed up.
// Replaced by the tests.
var StopKillMargin = 5 * time.Second

// ShutdownWait returns how long a stop waits for a container to shut down
// before forcing it: its stop_timeout, shortened to leave StopKillMargin
// before the deadline of ctx, the one of the action, which the stop_timeout
// of the object sets. Zero when there is no time left for a wait: the stop
// then forces the container at once.
func ShutdownWait(ctx context.Context, stopTimeout time.Duration) time.Duration {
	d := stopTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if left := time.Until(deadline) - StopKillMargin; left < d {
			d = left
		}
	}
	if d < 0 {
		return 0
	}
	return d
}
