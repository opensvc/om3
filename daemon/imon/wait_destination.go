package imon

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
)

// defaultWaitDestinationTimeout bounds the wait of a switch source for its
// destination to be ready to start when the object sets no start timeout.
const defaultWaitDestinationTimeout = time.Hour

// waitDestinationTimeout is how long a switch source waits for the
// destination to be ready to start before it stops: the start timeout of the
// object, the time a start may take.
func (t *Manager) waitDestinationTimeout() time.Duration {
	if t.instConfig.ActorConfig != nil && t.instConfig.ActorConfig.StartTimeout > 0 {
		return t.instConfig.ActorConfig.StartTimeout
	}
	return defaultWaitDestinationTimeout
}

// startBlockers returns the blockers the instance of a node publishes, by
// resource id.
func (t *Manager) startBlockers(nodename string) map[string]*resource.StartBlocker {
	m := make(map[string]*resource.StartBlocker)
	for rid, rstat := range t.instStatus[nodename].Resources {
		if rstat.StartBlocker != nil {
			m[rid] = rstat.StartBlocker
		}
	}
	return m
}

// sortedRIDs returns the resource ids of a blocker map, sorted.
func sortedRIDs(m map[string]*resource.StartBlocker) []string {
	l := make([]string, 0, len(m))
	for rid := range m {
		l = append(l, rid)
	}
	sort.Strings(l)
	return l
}

// blockerReasons says the blockers of the nodes, one per resource.
func (t *Manager) blockerReasons(nodenames []string) []string {
	l := make([]string, 0)
	for _, nodename := range nodenames {
		m := t.startBlockers(nodename)
		for _, rid := range sortedRIDs(m) {
			l = append(l, fmt.Sprintf("%s %s: %s", nodename, rid, m[rid].Reason))
		}
	}
	return l
}

// unliftableBlockers says the blockers of the destination no resync lifts:
// those a resync does not lift, as a disk the node does not have, and those
// no other node can resync from, the resource up to date nowhere else.
func (t *Manager) unliftableBlockers(dst []string) []string {
	l := make([]string, 0)
	for _, nodename := range dst {
		m := t.startBlockers(nodename)
		for _, rid := range sortedRIDs(m) {
			b := m[rid]
			switch {
			case !b.Resync:
				l = append(l, fmt.Sprintf("%s %s: %s", nodename, rid, b.Reason))
			case !t.isUpToDateElsewhere(rid, dst):
				l = append(l, fmt.Sprintf("%s %s: %s, and no other node holds it up to date to resync from", nodename, rid, b.Reason))
			}
		}
	}
	return l
}

// isUpToDateElsewhere says whether a node out of the destination holds the
// resource up, with nothing blocking its start: a source a resync can copy.
func (t *Manager) isUpToDateElsewhere(rid string, dst []string) bool {
	for nodename, instStatus := range t.instStatus {
		if slices.Contains(dst, nodename) {
			continue
		}
		rstat, ok := instStatus.Resources[rid]
		if !ok || rstat.StartBlocker != nil {
			continue
		}
		if rstat.Status.Is(status.Up, status.StandbyUp) {
			return true
		}
	}
	return false
}

// setWaitDestination has a switch source wait, before it stops, for the
// destination to be ready to start: a replicated disk the destination holds
// not up to date is resynced while the source runs, rather than after it
// stopped, which made the switch fail on a promotion refused, or start the
// destination as late as the resync took.
//
// It returns true while the source waits. The wait ends when the destination
// publishes no blocker, and fails, the source left as it was, when the
// destination fails to prepare, publishes a blocker no resync lifts, or is
// not ready after the start timeout of the object.
func (t *Manager) setWaitDestination() bool {
	dst, ok := t.getPlacedAtDestination()
	if !ok || len(dst) == 0 {
		return false
	}
	for _, nodename := range dst {
		if t.instMonitor[nodename].State == instance.MonitorStatePrepareFailure {
			t.failWaitDestination(fmt.Sprintf("%s failed to prepare the start", nodename))
			return true
		}
		if _, ok := t.instStatus[nodename]; !ok {
			// A destination publishing no status, as a node lost, is
			// not known ready: the source is left as it is.
			t.failWaitDestination(fmt.Sprintf("%s publishes no instance status", nodename))
			return true
		}
	}
	reasons := t.blockerReasons(dst)
	if len(reasons) == 0 {
		if t.state.State == instance.MonitorStateWaitDestination {
			t.log.Infof("destination %s ready to start", strings.Join(dst, ","))
			t.waitDestinationSince = time.Time{}
			t.waitDestinationTimer.Stop()
			t.transitionTo(instance.MonitorStateIdle)
		}
		return false
	}
	if l := t.unliftableBlockers(dst); len(l) > 0 {
		t.failWaitDestination(strings.Join(l, "; "))
		return true
	}
	if t.state.State != instance.MonitorStateWaitDestination {
		t.log.Infof("wait for %s to be ready to start before stopping, %s at most: %s",
			strings.Join(dst, ","), t.waitDestinationTimeout(), strings.Join(reasons, "; "))
		t.waitDestinationSince = time.Now()
		t.waitDestinationTimer.Reset(t.waitDestinationTimeout())
		t.transitionTo(instance.MonitorStateWaitDestination)
		return true
	}
	if time.Since(t.waitDestinationSince) > t.waitDestinationTimeout() {
		t.failWaitDestination(fmt.Sprintf("not ready to start after %s: %s", t.waitDestinationTimeout(), strings.Join(reasons, "; ")))
		return true
	}
	return true
}

// failWaitDestination ends the placement the source waited for the
// destination of, nothing stopped: the failed state says the orchestration
// failed, and the instance goes back to idle when it ends.
func (t *Manager) failWaitDestination(reason string) {
	t.loggerWithState().Errorf("orchestration %s: destination %s: the instance is not stopped", t.state.GlobalExpect, reason)
	t.waitDestinationSince = time.Time{}
	t.waitDestinationTimer.Stop()
	t.transitionTo(instance.MonitorStateWaitDestinationFailure)
	t.idleAfterOrchestration = true
	t.done()
	t.clearPending()
}

// onWaitDestinationTimer has the orchestration look again at the
// destination it waits for, its time being up, when no status change did.
func (t *Manager) onWaitDestinationTimer() {
	if t.state.State != instance.MonitorStateWaitDestination {
		return
	}
	t.onChange()
}

// resyncBlockerPaths returns the objects whose standby start starts the
// resync lifting the blockers of the local instance, this object for a
// blocker of its own.
func (t *Manager) resyncBlockerPaths() []string {
	l := make([]string, 0)
	m := t.startBlockers(t.localhost)
	for _, rid := range sortedRIDs(m) {
		b := m[rid]
		if !b.Resync {
			continue
		}
		p := b.Path
		if p == "" {
			p = t.path.String()
		}
		if !slices.Contains(l, p) {
			l = append(l, p)
		}
	}
	return l
}

// crmPrepareFunc starts standby the objects holding the resources the
// destination of a switch resyncs, which reconnects a replicated disk left
// behind without promoting it.
func (t *Manager) crmPrepareFunc(paths []string) func() error {
	return func() error {
		for _, p := range paths {
			if err := t.crmAction("prepare", p, "instance", "startstandby"); err != nil {
				return err
			}
		}
		return nil
	}
}
