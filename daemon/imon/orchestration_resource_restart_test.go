package imon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
)

// newRestartTestManager returns the manager of a local instance wanted
// started, its avail local, and the instance of node2 avail peer.
func newRestartTestManager(local, peer status.T) *Manager {
	m := newTestManager(&pubSpy{})
	m.state.LocalExpect = instance.MonitorLocalExpectStarted
	m.nodeMonitor = map[string]node.Monitor{"node1": {State: node.MonitorStateIdle}}
	m.nodeStatus = map[string]node.Status{"node1": {}}
	m.instStatus["node1"] = instance.Status{Avail: local, Provisioned: provisioned.True, UpdatedAt: time.Now()}
	m.instStatus["node2"] = instance.Status{Avail: peer, Provisioned: provisioned.True, UpdatedAt: time.Now()}
	return m
}

// A failover instance down while a peer runs the object is no longer the one
// started: restarting its resources would run the object twice.
func TestAFailoverInstanceDownIsNotRestartedWhileAPeerRunsTheObject(t *testing.T) {
	m := newRestartTestManager(status.Down, status.Up)
	m.orchestrateResourceRestart()
	assert.Equal(t, instance.MonitorLocalExpectNone, m.state.LocalExpect)
}

// The instance running the object, or the one of an object down everywhere,
// stays the one started.
func TestAFailoverInstanceStaysStartedWhenNoPeerRunsTheObject(t *testing.T) {
	for _, c := range []struct{ local, peer status.T }{
		{status.Up, status.Down},
		{status.Down, status.Down},
		{status.Warn, status.Down},
	} {
		m := newRestartTestManager(c.local, c.peer)
		m.orchestrateResourceRestart()
		assert.Equalf(t, instance.MonitorLocalExpectStarted, m.state.LocalExpect, "local %s, peer %s", c.local, c.peer)
	}
}

// A flex object runs on several nodes: a peer up says nothing of this one.
func TestAFlexInstanceStaysStartedWhileAPeerRuns(t *testing.T) {
	m := newRestartTestManager(status.Down, status.Up)
	m.objStatus.Topology = topology.Flex
	m.orchestrateResourceRestart()
	assert.Equal(t, instance.MonitorLocalExpectStarted, m.state.LocalExpect)
}
