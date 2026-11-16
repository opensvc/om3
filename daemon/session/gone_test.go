package session

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

// A purge whose participants all left is over only once the object is gone:
// n2, whose instance came back as it fetched the config again just before,
// was no participant yet as n1 left, and the purge was said over with its
// instance left (qatests 308).
func TestAPurgeEndsOnceTheObjectIsGone(t *testing.T) {
	reset()
	m := &Manager{}
	id := uuid.New()
	p := naming.Path{Namespace: "c308", Name: "s1", Kind: naming.KindSvc}
	t.Cleanup(func() {
		instance.ConfigData.Unset(p, "n2")
		awaitingGone = make(map[string]time.Time)
	})
	m.handle(&msgbus.InstanceMonitorUpdated{Path: p, Node: "n1", Value: instance.Monitor{OrchestrationID: id, GlobalExpect: instance.MonitorGlobalExpectPurged}})
	instance.ConfigData.Set(p, "n2", &instance.Config{})
	node.MonitorData.Set("n2", &node.Monitor{State: node.MonitorStateIdle})
	t.Cleanup(func() { node.MonitorData.Unset("n2") })

	m.handle(&msgbus.InstanceMonitorDeleted{Path: p, Node: "n1"})
	o, _ := GetOrchestration(id.String())
	assert.Equal(t, StateRunning, o.State, "n2 holds an instance still")

	instance.ConfigData.Unset(p, "n2")
	m.handle(&msgbus.InstanceConfigDeleted{Path: p, Node: "n2"})
	CheckGone()
	o, _ = GetOrchestration(id.String())
	assert.Equal(t, StateSucceeded, o.State, "the object is gone")
}

// An instance left past the timeout ends the purge failed, naming its node;
// one of a node in maintenance is not waited for: it catches up as it
// rejoins.
func TestAPurgeWithAnInstanceLeft(t *testing.T) {
	reset()
	m := &Manager{}
	p := naming.Path{Namespace: "c308", Name: "s2", Kind: naming.KindSvc}
	t.Cleanup(func() {
		instance.ConfigData.Unset(p, "n2")
		instance.ConfigData.Unset(p, "n3")
		node.MonitorData.Unset("n3")
		awaitingGone = make(map[string]time.Time)
	})
	instance.ConfigData.Set(p, "n3", &instance.Config{})
	node.MonitorData.Set("n3", &node.Monitor{State: node.MonitorStateMaintenance})

	id := uuid.New()
	m.handle(&msgbus.InstanceMonitorUpdated{Path: p, Node: "n1", Value: instance.Monitor{OrchestrationID: id, GlobalExpect: instance.MonitorGlobalExpectPurged}})
	m.handle(&msgbus.InstanceMonitorDeleted{Path: p, Node: "n1"})
	o, _ := GetOrchestration(id.String())
	assert.Equal(t, StateSucceeded, o.State, "n3 in maintenance is not waited for")

	instance.ConfigData.Set(p, "n2", &instance.Config{})
	node.MonitorData.Set("n2", &node.Monitor{State: node.MonitorStateIdle})
	t.Cleanup(func() { node.MonitorData.Unset("n2") })
	id = uuid.New()
	m.handle(&msgbus.InstanceMonitorUpdated{Path: p, Node: "n1", Value: instance.Monitor{OrchestrationID: id, GlobalExpect: instance.MonitorGlobalExpectPurged}})
	m.handle(&msgbus.InstanceMonitorDeleted{Path: p, Node: "n1"})
	o, _ = GetOrchestration(id.String())
	assert.Equal(t, StateRunning, o.State)
	mu.Lock()
	awaitingGone[id.String()] = time.Now().Add(-goneTimeout)
	mu.Unlock()
	CheckGone()
	o, _ = GetOrchestration(id.String())
	assert.Equal(t, StateFailed, o.State)
	assert.Contains(t, o.Error, "instances remain on n2")
}

// A node without a monitor, forgotten or not heard from since the daemon
// started, and a node upgrading, are not waited for.
func TestAPurgeDoesNotWaitForANodeWithoutMonitorOrUpgrading(t *testing.T) {
	reset()
	m := &Manager{}
	p := naming.Path{Namespace: "c308", Name: "s3", Kind: naming.KindSvc}
	t.Cleanup(func() {
		instance.ConfigData.Unset(p, "n2")
		instance.ConfigData.Unset(p, "n3")
		node.MonitorData.Unset("n3")
		awaitingGone = make(map[string]time.Time)
	})
	instance.ConfigData.Set(p, "n2", &instance.Config{})
	instance.ConfigData.Set(p, "n3", &instance.Config{})
	node.MonitorData.Set("n3", &node.Monitor{State: node.MonitorStateUpgrade})
	id := uuid.New()
	m.handle(&msgbus.InstanceMonitorUpdated{Path: p, Node: "n1", Value: instance.Monitor{OrchestrationID: id, GlobalExpect: instance.MonitorGlobalExpectPurged}})
	m.handle(&msgbus.InstanceMonitorDeleted{Path: p, Node: "n1"})
	o, _ := GetOrchestration(id.String())
	assert.Equal(t, StateSucceeded, o.State)
}

// The end the node that accepted a purge announces as its own instance goes
// does not end it while the object is not gone: it ends once the last
// instance goes, as the participants leaving does.
func TestTheEndOfAPurgeAnnouncedDoesNotSkipTheWaitForTheObjectGone(t *testing.T) {
	reset()
	m := &Manager{}
	id := uuid.New()
	p := naming.Path{Namespace: "c308", Name: "s2", Kind: naming.KindSvc}
	t.Cleanup(func() {
		instance.ConfigData.Unset(p, "n2")
		awaitingGone = make(map[string]time.Time)
	})
	node.MonitorData.Set("n2", &node.Monitor{State: node.MonitorStateIdle})
	t.Cleanup(func() { node.MonitorData.Unset("n2") })
	m.handle(&msgbus.ObjectOrchestrationAccepted{ID: id.String(), Node: "n1", Path: p, GlobalExpect: instance.MonitorGlobalExpectPurged})
	m.handle(&msgbus.InstanceMonitorUpdated{Path: p, Node: "n1", Value: instance.Monitor{OrchestrationID: id, GlobalExpect: instance.MonitorGlobalExpectPurged}})
	instance.ConfigData.Set(p, "n2", &instance.Config{})

	// Announced before the monitor of n1 is seen deleted, and after.
	m.handle(&msgbus.ObjectOrchestrationEnd{ID: id.String(), Node: "n1", Path: p, GlobalExpect: instance.MonitorGlobalExpectPurged})
	o, _ := GetOrchestration(id.String())
	assert.Equal(t, StateRunning, o.State, "n1 is a participant still")
	m.handle(&msgbus.InstanceMonitorDeleted{Path: p, Node: "n1"})
	m.handle(&msgbus.ObjectOrchestrationEnd{ID: id.String(), Node: "n1", Path: p, GlobalExpect: instance.MonitorGlobalExpectPurged})
	o, _ = GetOrchestration(id.String())
	assert.Equal(t, StateRunning, o.State, "n2 holds an instance still")

	instance.ConfigData.Unset(p, "n2")
	m.handle(&msgbus.InstanceConfigDeleted{Path: p, Node: "n2"})
	CheckGone()
	o, _ = GetOrchestration(id.String())
	assert.Equal(t, StateSucceeded, o.State, "the object is gone")
}
