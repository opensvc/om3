package imon

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

func mustParsePath(t *testing.T, s string) naming.Path {
	t.Helper()
	p, err := naming.ParsePath(s)
	require.NoError(t, err)
	return p
}

// setUser publishes the instance configuration and status of an object using
// others on a node, as that node evaluates them, and unsets them at the end of
// the test.
func setUser(t *testing.T, p naming.Path, node string, unprovisionDisabled bool, prov provisioned.T, uses ...string) {
	t.Helper()
	instance.ConfigData.Set(p, node, &instance.Config{ActorConfig: &instance.ActorConfig{UnprovisionDisabled: unprovisionDisabled, Uses: uses}})
	instance.StatusData.Set(p, node, &instance.Status{Avail: status.Down, Provisioned: prov})
	t.Cleanup(func() {
		instance.ConfigData.Unset(p, node)
		instance.StatusData.Unset(p, node)
	})
}

// askFor sends the monitor of the object a request for the global expect,
// and returns the reason it is refused for, empty when it is accepted.
func askFor(t *testing.T, p naming.Path, globalExpect instance.MonitorGlobalExpect) string {
	t.Helper()
	pub := &pubSpy{}
	m := newTestManager(pub)
	m.path = p
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	id := uuid.New()
	m.onSetInstanceMonitor(&msgbus.SetInstanceMonitor{
		Path:  p,
		Node:  m.localhost,
		Value: instance.MonitorUpdate{GlobalExpect: &globalExpect, CandidateOrchestrationID: id},
	})
	for _, msg := range pub.msgs {
		if v, ok := msg.(*msgbus.ObjectOrchestrationRefused); ok && v.ID == id.String() {
			assert.NotEqual(t, id, m.state.OrchestrationID, "a refused request takes no id")
			return v.Reason
		}
	}
	assert.Equal(t, id, m.state.OrchestrationID, "%s of %s accepted", globalExpect, p)
	return ""
}

// A purge or a delete of a cfg, a sec or a vol an object leaving its
// unprovision to the sysadmin uses is refused upfront, naming that object and
// the nodes it is provisioned on with unprovision=false, as each node
// evaluates the keyword: a scoped unprovision@node protects on that node
// alone. An instance the sysadmin freed protects nothing any more.
func TestAPurgeOrADeleteOfWhatAProtectedObjectUsesIsRefusedUpfront(t *testing.T) {
	user := mustParsePath(t, "repro/svc/d")
	cfg := mustParsePath(t, "repro/cfg/d")
	m := &Manager{path: cfg}

	// unprovision@n1 = false: n1 alone protects.
	setUser(t, user, "n1", true, provisioned.True, "repro/cfg/d", "repro/vol/d-data")
	setUser(t, user, "n2", false, provisioned.True, "repro/cfg/d", "repro/vol/d-data")
	reason := m.usedByProtectedRefusal()
	assert.Contains(t, reason, "used by repro/svc/d on n1")
	assert.Contains(t, reason, "unprovision=false")
	assert.NotContains(t, reason, "n2")

	// unprovision@n2 = false: n2 alone protects.
	setUser(t, user, "n1", false, provisioned.True, "repro/cfg/d", "repro/vol/d-data")
	setUser(t, user, "n2", true, provisioned.True, "repro/cfg/d", "repro/vol/d-data")
	assert.Contains(t, m.usedByProtectedRefusal(), "used by repro/svc/d on n2")

	// A node not using it there does not protect it.
	setUser(t, user, "n2", true, provisioned.True, "repro/vol/d-data")
	assert.Empty(t, m.usedByProtectedRefusal(), "n2 does not use it")

	// Freed by the sysadmin, and declared so.
	setUser(t, user, "n2", true, provisioned.False, "repro/cfg/d", "repro/vol/d-data")
	assert.Empty(t, m.usedByProtectedRefusal(), "freed by the sysadmin")

	// Every orchestration path: the purge and the delete are refused.
	setUser(t, user, "n1", true, provisioned.True, "repro/cfg/d", "repro/vol/d-data")
	assert.Contains(t, askFor(t, cfg, instance.MonitorGlobalExpectPurged), "used by repro/svc/d on n1")
	assert.Contains(t, askFor(t, cfg, instance.MonitorGlobalExpectDeleted), "used by repro/svc/d on n1")
	assert.Contains(t, askFor(t, mustParsePath(t, "repro/vol/d-data"), instance.MonitorGlobalExpectPurged), "used by repro/svc/d on n1")
	assert.Empty(t, askFor(t, cfg, instance.MonitorGlobalExpectFrozen), "a freeze is no purge")
}

// The refusal holds whoever asks: the admin of the namespace of a datastore
// is refused its purge and its delete while an object of another namespace,
// which that admin may not even see, uses it with unprovision=false. The
// monitor judges what the cluster holds, not the grants of the requester.
func TestANamespaceAdminIsRefusedThePurgeOfADatastoreAProtectedObjectOfAnotherNamespaceUses(t *testing.T) {
	setUser(t, mustParsePath(t, "other/svc/web"), "n1", true, provisioned.True, "shared/sec/web-*")
	shared := mustParsePath(t, "shared/sec/web-front")
	assert.Contains(t, askFor(t, shared, instance.MonitorGlobalExpectPurged), "used by other/svc/web on n1")
	assert.Contains(t, askFor(t, shared, instance.MonitorGlobalExpectDeleted), "used by other/svc/web on n1")
	assert.Empty(t, askFor(t, mustParsePath(t, "shared/sec/db"), instance.MonitorGlobalExpectPurged), "not matched by the pattern")
}

// A namespace-wide purge, as om 'repro/**' purge, asks the monitor of each
// object: the protected service and what it uses are refused, each with its
// reason, the other objects of the namespace are purged.
func TestANamespacePurgeRefusesTheProtectedDependenciesAndPurgesTheOthers(t *testing.T) {
	setUser(t, mustParsePath(t, "repro/svc/d"), "n1", true, provisioned.True, "repro/cfg/d", "repro/sec/d", "repro/vol/d-data")
	setUser(t, mustParsePath(t, "repro/svc/free"), "n1", false, provisioned.True, "repro/cfg/free", "repro/vol/free-data")
	setUser(t, mustParsePath(t, "repro/svc/freed"), "n1", true, provisioned.False, "repro/cfg/freed")

	refused := make(map[string]string)
	for _, s := range []string{
		"repro/svc/d", "repro/cfg/d", "repro/sec/d", "repro/vol/d-data",
		"repro/svc/free", "repro/cfg/free", "repro/vol/free-data",
		"repro/svc/freed", "repro/cfg/freed", "repro/cfg/unused",
	} {
		p := mustParsePath(t, s)
		var reason string
		if instance.ConfigData.GetByPathAndNode(p, "n1") != nil {
			// The protected service itself is refused by its own
			// unprovision=false.
			m := &Manager{path: p}
			reason = m.provisionDisabledRefusal(instance.MonitorGlobalExpectPurged)
		} else {
			reason = askFor(t, p, instance.MonitorGlobalExpectPurged)
		}
		if reason != "" {
			refused[s] = reason
		}
	}
	assert.Len(t, refused, 4, "%v", refused)
	assert.Contains(t, refused["repro/svc/d"], "unprovision is disabled")
	for _, s := range []string{"repro/cfg/d", "repro/sec/d", "repro/vol/d-data"} {
		assert.Contains(t, refused[s], "used by repro/svc/d on n1", s)
	}
}

// A vol a stopped service leaving its unprovision to the sysadmin holds,
// through its claim, is never released by an orchestration: its unprovision
// and its purge end failed with the reason, rather than wait for ever. The
// verdict of the orchestration says it. A child unprovision=true is still
// waited for.
func TestATeardownOfAVolHeldByAProtectedChildEndsFailedWithTheReason(t *testing.T) {
	child := mustParsePath(t, "repro/svc/d")
	relation := child.String() + "@node1"
	newManager := func(globalExpect instance.MonitorGlobalExpect) *Manager {
		m := newTestManager(&pubSpy{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		m.ctx = ctx
		m.path = mustParsePath(t, "repro/vol/d-data")
		m.scopeNodes = []string{"node1"}
		m.state.GlobalExpect = globalExpect
		m.state.OrchestrationID = uuid.New()
		m.state.Children = map[string]status.T{relation: status.Down}
		m.instStatus[m.localhost] = instance.Status{Avail: status.Down, Provisioned: provisioned.True}
		return m
	}

	setUser(t, child, "node1", false, provisioned.True, "repro/vol/d-data")
	m := newManager(instance.MonitorGlobalExpectPurged)
	m.orchestratePurged()
	assert.Equal(t, instance.MonitorStateWaitChildren, m.state.State, "a child unprovision=true is waited for")
	assert.False(t, m.state.OrchestrationIsDone)

	setUser(t, child, "node1", true, provisioned.True, "repro/vol/d-data")
	m = newManager(instance.MonitorGlobalExpectPurged)
	m.orchestratePurged()
	assert.Equal(t, instance.MonitorStatePurgeFailed, m.state.State)
	assert.True(t, m.state.OrchestrationIsDone, "the purge ends")

	m.setNextPendingOrchestration()
	m.setOrchestrationVerdict()
	require.NotNil(t, m.orchestrationPending)
	assert.True(t, m.orchestrationPending.Failed)
	assert.Contains(t, m.orchestrationPending.Error, "purge failed on node1: repro/svc/d@node1 is provisioned with its unprovision disabled (unprovision=false)")

	m = newManager(instance.MonitorGlobalExpectUnprovisioned)
	m.state.State = instance.MonitorStateWaitChildren
	m.orchestrateUnprovisioned()
	assert.Equal(t, instance.MonitorStateUnprovisionFailure, m.state.State, "a wait begun before ends too")
	assert.True(t, m.state.OrchestrationIsDone, "the unprovision ends")
	m.orchestrateUnprovisioned()
	assert.Equal(t, instance.MonitorStateUnprovisionFailure, m.state.State, "the failure lingers")

	setUser(t, child, "node1", true, provisioned.False, "repro/vol/d-data")
	m = newManager(instance.MonitorGlobalExpectPurged)
	assert.False(t, m.setWaitChildren(), "freed by the sysadmin, the child holds nothing")
}

// An object using the datastore, unprovisioned but being provisioned, holds
// it: its provision would read what the purge takes away.
func TestAnObjectBeingProvisionedProtectsWhatItUses(t *testing.T) {
	user := mustParsePath(t, "repro/svc/coming")
	cfg := mustParsePath(t, "repro/cfg/coming")
	m := &Manager{path: cfg}
	setUser(t, user, "n1", true, provisioned.False, "repro/cfg/coming")
	assert.Empty(t, m.usedByProtectedRefusal(), "unprovisioned and idle, it holds nothing")

	instance.MonitorData.Set(user, "n1", &instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectProvisioned, State: instance.MonitorStateProvisionProgress})
	t.Cleanup(func() { instance.MonitorData.Unset(user, "n1") })
	assert.Contains(t, m.usedByProtectedRefusal(), "repro/svc/coming on n1")
}

// A failed action is terminal: an unprovisioned instance whose provision
// failed holds nothing, and blocks no purge for good.
func TestAnObjectWhoseProvisionFailedProtectsNothing(t *testing.T) {
	user := mustParsePath(t, "repro/svc/failed")
	cfg := mustParsePath(t, "repro/cfg/failed")
	m := &Manager{path: cfg}
	setUser(t, user, "n1", true, provisioned.False, "repro/cfg/failed")
	instance.MonitorData.Set(user, "n1", &instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectProvisioned, State: instance.MonitorStateProvisionFailure})
	t.Cleanup(func() { instance.MonitorData.Unset(user, "n1") })
	assert.Empty(t, m.usedByProtectedRefusal())
}
