package imon

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
)

// newWarnTestManager returns the manager on localhost of a failover object
// in warn, its instance on node2 up in part, as a sync restore --stop leaves
// it: disks up, apps down.
func newWarnTestManager(localhost string) *Manager {
	m := newAffinityTestManager(instance.ActorConfig{})
	m.localhost = localhost
	m.objStatus.Topology = topology.Failover
	m.objStatus.Avail = status.Warn
	m.objStatus.Provisioned = provisioned.True
	m.instStatus["node2"] = instance.Status{Avail: status.Warn, Provisioned: provisioned.True}
	m.state.GlobalExpect = instance.MonitorGlobalExpectStarted
	return m
}

// A start asked of a failover object one instance of which is up in part is
// accepted, as v2 did, and starts that instance only: the others would start
// next to it on the same disks.
func TestARequestedStartCompletesTheInstanceInWarn(t *testing.T) {
	m := newWarnTestManager("node2")
	ok, reason := m.isRequestedStartable()
	assert.True(t, ok, reason)
	assert.True(t, m.isStartCandidateLeader(false), "the instance in warn starts")

	m = newWarnTestManager("node1")
	assert.False(t, m.isStartCandidateLeader(false), "node1, first in placement, waits for the instance in warn")
}

// Instances up on two nodes are a split: the start stays refused.
func TestARequestedStartRefusesASplit(t *testing.T) {
	m := newWarnTestManager("node1")
	m.instStatus["node1"] = instance.Status{Avail: status.Up, Provisioned: provisioned.True}
	ok, reason := m.isRequestedStartable()
	assert.False(t, ok)
	assert.Contains(t, reason, "up on node1, node2")
}

// The instance in warn must be able to start, or no instance may.
func TestARequestedStartRefusesAnInstanceInWarnThatCannotStart(t *testing.T) {
	m := newWarnTestManager("node1")
	m.instStatus["node2"] = instance.Status{Avail: status.Warn, Provisioned: provisioned.Mixed}
	ok, reason := m.isRequestedStartable()
	assert.False(t, ok)
	assert.Contains(t, reason, "partly up on node2")
	assert.False(t, m.isStartCandidateLeader(false))
}

// The daemon starting on its own still refuses a failover object in warn.
func TestTheDaemonDoesNotStartAFailoverObjectInWarn(t *testing.T) {
	m := newWarnTestManager("node2")
	ok, _ := m.isHAOrchestrateable()
	assert.False(t, ok)
}

// A giveback moves the object to its placement leader, an instance in warn
// elsewhere included.
func TestAGivebackOfAnObjectInWarnGoesToThePlacementLeader(t *testing.T) {
	m := newWarnTestManager("node1")
	m.state.GlobalExpect = instance.MonitorGlobalExpectPlaced
	assert.True(t, m.isStartCandidateLeader(false), "node1 leads the placement")
}
