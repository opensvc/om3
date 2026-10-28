package imon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/node"
)

// During a cluster thaw, a peer still frozen whose node monitor carries the
// thaw is a candidate already: node2, thawed first, does not take the
// leadership node1 has once thawed, which would have it ready the instances
// node1 readies too, and node1 wait its ready period again.
func TestAThawingPeerKeepsTheHALeadership(t *testing.T) {
	m := newAffinityTestManager(instance.ActorConfig{})
	m.localhost = "node2"
	m.nodeStatus["node1"] = node.Status{FrozenAt: time.Now()}

	assert.True(t, m.newIsHALeader(), "a peer frozen, and not being thawed, is no candidate")

	m.nodeMonitor["node1"] = node.Monitor{State: node.MonitorStateUnfreezeProgress, GlobalExpect: node.MonitorGlobalExpectUnfrozen}
	assert.False(t, m.newIsHALeader(), "a peer being thawed is a candidate, placed first")

	m.nodeMonitor["node1"] = node.Monitor{State: node.MonitorStateUnfreezeFailure, GlobalExpect: node.MonitorGlobalExpectUnfrozen}
	assert.True(t, m.newIsHALeader(), "a peer whose thaw failed is no candidate")
}

// The local node takes no initiative while frozen, being thawed or not.
func TestTheLocalNodeBeingThawedIsNoCandidate(t *testing.T) {
	m := newAffinityTestManager(instance.ActorConfig{})
	m.nodeStatus["node1"] = node.Status{FrozenAt: time.Now()}
	m.nodeMonitor["node1"] = node.Monitor{State: node.MonitorStateUnfreezeProgress, GlobalExpect: node.MonitorGlobalExpectUnfrozen}
	assert.False(t, m.newIsHALeader())
}
