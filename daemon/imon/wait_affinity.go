package imon

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
)

// waitAffinityGrace is how long an orchestration starting the object waits
// for an object its hard affinities name to show it comes up, before it gives
// up: a request starting the object and the one starting that object, made a
// moment apart, are accepted a moment apart.
var waitAffinityGrace = 10 * time.Second

// goingUpGlobalExpects are the global expects of an orchestration bringing an
// object up somewhere.
var goingUpGlobalExpects = []instance.MonitorGlobalExpect{
	instance.MonitorGlobalExpectStarted,
	instance.MonitorGlobalExpectPlaced,
	instance.MonitorGlobalExpectPlacedAt,
	instance.MonitorGlobalExpectProvisioned,
}

// goingUpStates are the states of an instance on its way up, or waiting to
// be.
var goingUpStates = []instance.MonitorState{
	instance.MonitorStatePreflightProgress,
	instance.MonitorStatePreflightSuccess,
	instance.MonitorStateProvisionProgress,
	instance.MonitorStateStartProgress,
	instance.MonitorStateReady,
	instance.MonitorStateWaitParents,
	instance.MonitorStateWaitLeader,
	instance.MonitorStateWaitAffinity,
	instance.MonitorStateWaitFiles,
}

// failedUpStates are the states an instance ends on when it failed to come
// up.
var failedUpStates = []instance.MonitorState{
	instance.MonitorStateStartFailure,
	instance.MonitorStateProvisionFailure,
	instance.MonitorStateWaitAffinityFailure,
}

// hasHardAffinityNode says a node of nodes satisfies the hard affinities.
func (t *Manager) hasHardAffinityNode(nodes []string) bool {
	return slices.ContainsFunc(nodes, t.isHardAffinitySatisfied)
}

// affinityBlockers returns the objects the hard affinities wait for on
// nodes: those not up there, the object of a hard affinity cycle starting
// first for a follower of the cycle. The objects of the hard anti affinities
// are none: waiting for them is waiting for them to go down, which no
// orchestration starting the object asks.
func (t *Manager) affinityBlockers(nodes []string) naming.Paths {
	var l naming.Paths
	for _, node := range nodes {
		for _, v := range t.affinityViolations(node) {
			p := v.path
			if !v.follows.IsZero() {
				p = v.follows
			}
			if v.anti || p.IsZero() || slices.Contains(l, p) {
				continue
			}
			l = append(l, p)
		}
	}
	return l
}

// hardAffinityNeverMet says why the hard affinities can never let the object
// start on nodes, and "" when they may: an object they name does not exist,
// or has no instance on the nodes of the object, or the hard anti affinities
// refuse every node. An orchestration starting the object is refused then,
// as waiting would wait for nothing.
func (t *Manager) hardAffinityNeverMet(nodes []string) string {
	cfg := t.instConfig.ActorConfig
	if cfg == nil || len(nodes) == 0 {
		return ""
	}
	for _, p := range cfg.HardAffinity {
		if p == t.path {
			continue
		}
		affineCfg, ok := t.affinityConfig(p)
		if !ok {
			if len(instance.StatusData.GetNodesByPath(p)) == 0 {
				return fmt.Sprintf("hard affinity with %s: no such object", p)
			}
			continue
		}
		if len(affineCfg.Scope) > 0 && !slices.ContainsFunc(affineCfg.Scope, func(node string) bool { return slices.Contains(nodes, node) }) {
			return fmt.Sprintf("hard affinity with %s: it has no instance on %s", p, strings.Join(nodes, ", "))
		}
	}
	var anti []string
	for _, node := range nodes {
		refused := false
		for _, v := range t.affinityViolations(node) {
			if v.anti {
				refused = true
				anti = append(anti, v.text)
			}
		}
		if !refused {
			return ""
		}
	}
	return strings.Join(anti, ", ")
}

// affineIntent says whether the object p shows it comes up: an
// orchestration bringing it up runs, or an instance is on its way up. When it
// does not, failures lists its instances that failed to come up since the
// wait began.
func (t *Manager) affineIntent(p naming.Path) (trying bool, failures []string) {
	for node, mon := range instance.MonitorData.GetByPath(p) {
		if mon == nil {
			continue
		}
		if !mon.OrchestrationIsDone && slices.Contains(goingUpGlobalExpects, mon.GlobalExpect) {
			return true, nil
		}
		if mon.State.IsOneOf(goingUpStates...) {
			return true, nil
		}
		if mon.State.IsOneOf(failedUpStates...) && !mon.StateUpdatedAt.Before(t.waitAffinitySince) {
			failures = append(failures, fmt.Sprintf("%s on %s", mon.State, node))
		}
	}
	slices.Sort(failures)
	return false, failures
}

// setWaitAffinity has the instance wait, in MonitorStateWaitAffinity, for an
// object the hard affinities name to come up, as no node of nodes satisfies
// them, and says so. It says false when a node satisfies them, the wait over
// if there was one.
//
// The wait ends in MonitorStateWaitAffinityFailure, with the reason in the
// StateReason of the monitor, the client waiting for the orchestration to end
// printing it, when an object waited for failed to come up, or showed no
// intention to within waitAffinityGrace.
func (t *Manager) setWaitAffinity(nodes []string) bool {
	if cfg := t.instConfig.ActorConfig; cfg == nil || len(cfg.HardAffinity)+len(cfg.HardAntiAffinity) == 0 || len(nodes) == 0 {
		return false
	}
	if t.hasHardAffinityNode(nodes) {
		if t.state.State == instance.MonitorStateWaitAffinity {
			t.log.Infof("stop waiting affinity: a node satisfies the hard affinities")
			t.stopWaitAffinityTimer()
			t.transitionTo(instance.MonitorStateIdle)
		}
		return false
	}
	if t.state.State == instance.MonitorStateWaitAffinityFailure && t.waitAffinityFailedID == t.state.OrchestrationID {
		return true
	}
	if t.state.State != instance.MonitorStateWaitAffinity {
		t.waitAffinitySince = time.Now()
		t.log.Infof("wait affinity: %s", strings.Join(t.hardAffinityViolationsOf(nodes), ", "))
		t.transitionTo(instance.MonitorStateWaitAffinity)
	}
	var noIntention naming.Paths
	for _, p := range t.affinityBlockers(nodes) {
		trying, failures := t.affineIntent(p)
		switch {
		case trying:
			continue
		case len(failures) > 0:
			t.failWaitAffinity(fmt.Sprintf("hard affinity with %s: %s", p, strings.Join(failures, " and ")))
			return true
		default:
			noIntention = append(noIntention, p)
		}
	}
	if len(noIntention) == 0 {
		t.stopWaitAffinityTimer()
		return true
	}
	if left := waitAffinityGrace - time.Since(t.waitAffinitySince); left > 0 {
		t.waitAffinityTimer.Reset(left)
		return true
	}
	t.failWaitAffinity(fmt.Sprintf("hard affinity with %s: not up, and not coming up", noIntention))
	return true
}

// hardAffinityViolationsOf lists the reasons the hard affinities refuse each
// node of nodes.
func (t *Manager) hardAffinityViolationsOf(nodes []string) []string {
	var l []string
	for _, node := range nodes {
		l = append(l, t.hardAffinityViolations(node)...)
	}
	return l
}

// failWaitAffinity ends the wait for an object of the hard affinities on a
// failure, telling why.
func (t *Manager) failWaitAffinity(reason string) {
	t.log.Warnf("wait affinity failed: %s", reason)
	t.stopWaitAffinityTimer()
	t.waitAffinityFailedID = t.state.OrchestrationID
	t.state.StateReason = reason
	t.transitionTo(instance.MonitorStateWaitAffinityFailure)
}

// stopWaitAffinityTimer stops the timer of the grace of a wait for an object
// of the hard affinities.
func (t *Manager) stopWaitAffinityTimer() {
	if t.waitAffinityTimer != nil {
		t.waitAffinityTimer.Stop()
	}
}

// isWaitAffinityFailureOfOther says the instance ended a wait for an object
// of the hard affinities on a failure for another orchestration than the
// current one: the state lingers for the operator, and a new orchestration
// starts from it as from idle.
func (t *Manager) isWaitAffinityFailureOfOther() bool {
	return t.state.State == instance.MonitorStateWaitAffinityFailure && t.waitAffinityFailedID != t.state.OrchestrationID
}

// onAffinityChange reevaluates a wait for an object of the hard affinities
// when an object they name changed, its monitor or its status.
func (t *Manager) onAffinityChange(p naming.Path) {
	if _, ok := t.affinitySubscribed[p]; !ok {
		return
	}
	if t.state.State == instance.MonitorStateWaitAffinity {
		t.onChange()
	}
}
