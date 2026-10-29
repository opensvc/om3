package imon

import (
	"fmt"
	"sort"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
)

// provisionedFailureStates are the states the work of a provisioned
// orchestration ends on when it fails.
//
// The orchestration provisions what needs provisioning and starts what does
// not, so a failed start is its own failure, as terminal as a failed
// provision: the instance monitor retries neither, and an orchestration that
// does not end on them never ends.
var provisionedFailureStates = []instance.MonitorState{
	instance.MonitorStateProvisionFailure,
	instance.MonitorStateStartFailure,
}

func (t *Manager) orchestrateProvisioned() {
	switch t.state.State {
	case instance.MonitorStateIdle,
		instance.MonitorStateStopFailure,
		instance.MonitorStateUnfreezeSuccess,
		instance.MonitorStateUnprovisionFailure:
		t.provisionedFromIdle()
	case instance.MonitorStateStartSuccess:
		t.provisionedFromProvisioned()
	case instance.MonitorStateProvisionSuccess:
		t.provisionedFromProvisioned()
	case instance.MonitorStatePreflightSuccess:
		t.provisionedFromPreflighted()
	case instance.MonitorStateWaitLeader:
		t.provisionedFromWaitLeader()
	case instance.MonitorStateProvisionFailure:
		t.provisionedFromProvisionFailed()
	case instance.MonitorStateStartFailure:
		t.provisionedFromProvisionFailed()
	case instance.MonitorStateUnfreezeProgress:
	case instance.MonitorStateUnfreezeFailure:
		// TODO: clear ?
	case instance.MonitorStateWaitParents:
		t.setWaitParents()
	}
}

func (t *Manager) provisionedFromProvisioned() {
	if t.instStatus[t.localhost].IsFrozen() {
		t.doTransitionAction(t.unfreeze, instance.MonitorStateUnfreezeProgress, instance.MonitorStateUnfreezeSuccess, instance.MonitorStateUnfreezeFailure)
	} else {
		t.doneAndIdle()
	}
}

func (t *Manager) provisionedFromProvisionFailed() {
	if t.provisionedClearIfReached() {
		return
	}
}

func (t *Manager) provisionedFromIdle() {
	if t.setWaitParents() {
		return
	}
	// Every node checks, before any provisions or starts, that nothing on
	// it holds what the object would take: the leader would otherwise make
	// what the others would refuse, as a disk, or start what it adopted.
	//
	// The check comes before the orchestration tells it is reached: an
	// instance is found provisioned, and up, when what its resources name
	// exists, as a filesystem mounted on its mount point by something
	// else. What a provision made is not checked again, so an instance
	// its provision made passes, and the orchestration is reached then.
	if !t.provisionedOptions().Adopt {
		t.queueAction(t.crmProvisionPreflight, instance.MonitorStatePreflightProgress, instance.MonitorStatePreflightSuccess, instance.MonitorStateProvisionFailure)
		return
	}
	if t.provisionedClearIfReached() {
		return
	}
	t.provisionedFromChecked()
}

// provisionedFromPreflighted has the leader provision once every node
// checked, and the others wait for the leader. A node whose check failed ends
// the orchestration on every node: nothing is provisioned.
func (t *Manager) provisionedFromPreflighted() {
	if t.provisionedClearIfReached() {
		return
	}
	for node, instMon := range t.instMonitor {
		if node != t.localhost && instMon.State == instance.MonitorStateProvisionFailure {
			t.log.Infof("provision preflight failed on %s: provision nothing", node)
			t.transitionTo(instance.MonitorStateProvisionFailure)
			return
		}
	}
	if t.isProvisioningLeader() && !t.arePeersPreflighted() {
		return
	}
	t.provisionedFromChecked()
}

// arePeersPreflighted says whether every peer instance monitor has checked
// what the object would take on its node: it is past its check in the
// orchestration, or done with it, as a peer is once checked when the leader
// instance needs no provision, which it does not wait for.
func (t *Manager) arePeersPreflighted() bool {
	for node, instMon := range t.instMonitor {
		if node == t.localhost {
			continue
		}
		if instMon.OrchestrationIsDone && instMon.OrchestrationID == t.state.OrchestrationID {
			continue
		}
		if !instMon.State.IsOneOf(
			instance.MonitorStatePreflightSuccess,
			instance.MonitorStateWaitLeader,
			instance.MonitorStateStartProgress,
			instance.MonitorStateStartSuccess,
			instance.MonitorStateProvisionProgress,
			instance.MonitorStateProvisionSuccess,
		) {
			return false
		}
	}
	return true
}

// provisionedOptions returns the options of the provisioned orchestration.
func (t *Manager) provisionedOptions() (options instance.MonitorGlobalExpectOptionsProvisioned) {
	options, _ = t.state.GlobalExpectOptions.(instance.MonitorGlobalExpectOptionsProvisioned)
	return
}

// provisionedFromChecked has the leader provision, or start what needs no
// provision, and the others wait for it.
func (t *Manager) provisionedFromChecked() {
	if !t.isLocalStatusCurrent() {
		return
	}
	if t.isProvisioningLeader() {
		if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.NotApplicable, provisioned.True) {
			t.queueAction(t.crmStart, instance.MonitorStateStartProgress, instance.MonitorStateStartSuccess, instance.MonitorStateStartFailure)
		} else {
			t.queueAction(t.crmProvisionLeader, instance.MonitorStateProvisionProgress, instance.MonitorStateProvisionSuccess, instance.MonitorStateProvisionFailure)
		}
		return
	} else {
		t.transitionTo(instance.MonitorStateWaitLeader)
	}
}

func (t *Manager) provisionedFromWaitLeader() {
	if t.provisionedClearIfReached() {
		t.transitionTo(instance.MonitorStateIdle)
		return
	}
	if !t.hasLeaderProvisioned() || !t.isLocalStatusCurrent() {
		return
	}
	if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.NotApplicable, provisioned.True) {
		t.queueAction(t.crmStartStandby, instance.MonitorStateStartProgress, instance.MonitorStateStartSuccess, instance.MonitorStateStartFailure)
	} else {
		t.queueAction(t.crmProvisionNonLeader, instance.MonitorStateProvisionProgress, instance.MonitorStateProvisionSuccess, instance.MonitorStateProvisionFailure)
	}
	return
}

func (t *Manager) provisionedClearIfReached() bool {
	reached := func(msg string, succeed bool) bool {
		if succeed && t.isStopped() {
			// The object was flagged stopped when it was created, so the
			// daemon would not start it before it could run. It can now, and
			// the operator asking for a provision is asking for that.
			//
			// This used to unfreeze instead, on the freeze the creation had
			// set. Neither the freeze nor the unfreeze were the operator's,
			// and the pair of them left an object running with its failover
			// disabled whenever the second one did not happen.
			t.log.Infof("clear the stopped flag: the instance is provisioned")
			if err := t.unsetStopped(); err != nil {
				t.log.Errorf("clear the stopped flag: %s", err)
			}
		}
		if succeed {
			t.log.Infof("provisioned orchestration reached: %s", msg)
			t.doneAndIdle()
		} else {
			t.log.Infof("provisioned orchestration done: %s", msg)
			t.done()
		}
		if succeed && t.isLocalStarted() {
			t.enableMonitor("instance is now started")
		}
		t.updateIfChange()
		return true
	}

	// failures
	//
	// A start action can fail and leave the instance started anyway, a
	// timeout on an object whose resources did come up being the plain case.
	// The orchestration asked for a started instance and has one, so the
	// success conditions below have precedence, as they did before a start
	// failure ended this orchestration. The started orchestration draws the
	// same line in startedFromStartFailed.
	if t.state.State.IsOneOf(instance.MonitorStateStartFailure) && t.isLocalStarted() {
		t.log.Infof("instance is started, whatever its start reported")
	} else if t.isAllState(provisionedFailureStates...) {
		return reached("all instances failed to provision or start", false)
	} else if t.hasLeaderProvisionedFailed() {
		return reached("leader instance failed to provision or start", false)
	} else if t.state.State.IsOneOf(provisionedFailureStates...) {
		return reached(fmt.Sprintf("instance is %s", t.state.State), false)
	}

	if !t.isStarted() && !t.objStatus.Avail.Is(status.NotApplicable) {
		return false
	}
	if !t.isLocalStatusCurrent() {
		return false
	}

	// succeeds
	if t.instStatus[t.localhost].Provisioned == provisioned.True {
		return reached("instance is provisioned", true)
	} else if t.instStatus[t.localhost].Avail == status.NotApplicable {
		return reached("instance availability is n/a", true)
	} else if t.instStatus[t.localhost].Provisioned == provisioned.NotApplicable {
		if t.isProvisioningLeader() && t.instStatus[t.localhost].Avail.Is(status.Up) {
			return reached("unprovisionable leader instance is up", true)
		} else if !t.isProvisioningLeader() && t.instStatus[t.localhost].Avail.Is(status.Down, status.StandbyUp, status.StandbyUpWithDown, status.StandbyUpWithUp) {
			return reached("unprovisionable leader instance is up", true)
		}
	}
	return false
}

func (t *Manager) leaders() []string {
	l := make([]string, 0)
	for node, instMon := range t.instMonitor {
		if instMon.IsLeader {
			l = append(l, node)
		}
	}
	if t.state.IsLeader {
		l = append(l, t.localhost)
	}
	return l
}

// provisioningLeader returns one of all leaders.
// Select the first in alphalexical order.
//
// A failover object running somewhere is led from where it runs. The leader
// provisions and starts, the others provision and stay down: the placement
// leader, when the object runs elsewhere, would start a second instance of
// it.
func (t *Manager) provisioningLeader() string {
	if t.objStatus.Topology != topology.Flex {
		if running := t.runningInstances(); len(running) > 0 {
			sort.Strings(running)
			return running[0]
		}
	}
	leaders := t.leaders()
	switch len(leaders) {
	case 0:
		return ""
	case 1:
		return leaders[0]
	default:
		sort.StringSlice(leaders).Sort()
		return leaders[0]
	}
}

// runningInstances returns the nodes whose instance is up, or warn.
func (t *Manager) runningInstances() []string {
	l := make([]string, 0)
	for node, instStatus := range t.instStatus {
		if instStatus.Avail.Is(status.Up, status.Warn) {
			l = append(l, node)
		}
	}
	return l
}

func (t *Manager) isProvisioningLeader() bool {
	if t.objStatus.Topology == topology.Flex {
		return t.state.IsLeader
	} else {
		if t.provisioningLeader() == t.localhost {
			return true
		}
		return false
	}
}

// hasLeaderProvisioned says whether the instance leading the provision
// provisioned, so a node waiting for it provisions as a follower.
//
// The leader may change during the orchestration: an instance provisioned
// n/a, as one of a directory, is no leader candidate anymore, and a node
// waiting for the leader became the leader, waiting for itself. With the
// leader waiting for a leader, no node does the part of the leader, so a
// peer done with its part of the orchestration, provisioned, did it, or
// followed it: the leader provisioned. With the leader doing its part, a peer
// done tells nothing of it, as one already provisioned when the orchestration
// began.
func (t *Manager) hasLeaderProvisioned() bool {
	leader := t.provisioningLeader()
	if leaderInstanceStatus, ok := t.instStatus[leader]; ok && leaderInstanceStatus.Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
		return true
	}
	if !t.isWaitingLeader(leader) {
		return false
	}
	return t.hasPeerDoneProvisioned()
}

// isWaitingLeader says whether the instance of node waits for the leader of
// the provision.
func (t *Manager) isWaitingLeader(node string) bool {
	if node == t.localhost {
		return t.state.State == instance.MonitorStateWaitLeader
	}
	instMon, ok := t.instMonitor[node]
	return ok && instMon.State == instance.MonitorStateWaitLeader && instMon.OrchestrationID == t.state.OrchestrationID
}

// hasPeerDoneProvisioned says whether a peer instance is done with its part
// of the orchestration, provisioned.
func (t *Manager) hasPeerDoneProvisioned() bool {
	for node, instMon := range t.instMonitor {
		if node == t.localhost {
			continue
		}
		if !instMon.OrchestrationIsDone || instMon.OrchestrationID != t.state.OrchestrationID {
			continue
		}
		if t.instStatus[node].Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
			return true
		}
	}
	return false
}

func (t *Manager) hasLeaderProvisionedFailed() bool {
	leader := t.provisioningLeader()
	if t.instMonitor[leader].State.IsOneOf(provisionedFailureStates...) {
		return true
	}
	return false
}
