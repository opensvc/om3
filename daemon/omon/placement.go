package omon

import (
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/placement"
	"github.com/opensvc/om3/v3/core/status"
)

// placementState says whether an object runs where its placement policy puts
// it: optimal when its leaders are up and the other instances are not,
// non-optimal otherwise.
//
// The leaders are the natural ones, the instances the placement policy puts
// first, frozen or stopped on purpose as they may be. An object an operator
// runs elsewhere, frozen where the daemon would not move it, does not run
// where its placement puts it, which is what this says. The ha leaders, the
// instances the daemon may start on its own, leave the frozen and stopped
// instances out, so an object frozen on all its nodes has none to compare to.
//
// An object not up, or which is not a service, has no placement to judge.
func placementState(kind naming.Kind, avail status.T, monitors map[string]instance.Monitor, statuses map[string]instance.Status) placement.State {
	if kind != naming.KindSvc {
		return placement.NotApplicable
	}
	if avail.Is(status.Down, status.NotApplicable, status.Undef, status.Warn) {
		return placement.NotApplicable
	}
	state := placement.NotApplicable
	for node, monitor := range monitors {
		instStatus, ok := statuses[node]
		if !ok {
			return placement.NotApplicable
		}
		if monitor.IsLeader && !instStatus.Avail.Is(status.Up, status.NotApplicable) {
			return placement.NonOptimal
		}
		if !monitor.IsLeader && !instStatus.Avail.Is(status.Down, status.StandbyUp, status.StandbyDown, status.NotApplicable) {
			return placement.NonOptimal
		}
		state = placement.Optimal
	}
	return state
}
