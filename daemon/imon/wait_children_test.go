package imon

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
)

// A volume purged on a node where the service using it is down waits for the
// service instance to be unprovisioned or deleted, as it waits for a service
// up: deleted first, the volume takes away the mount point the unprovision of
// the share of the service reads. A service whose own purge failed releases
// it, and a volume started does not wait for a service down.
func TestAPurgedVolumeWaitsForItsProvisionedChildInstance(t *testing.T) {
	child := naming.Path{Namespace: "ns", Kind: naming.KindSvc, Name: "nfs"}
	relation := child.String() + "@node1"
	defer instance.StatusData.Unset(child, "node1")
	defer instance.MonitorData.Unset(child, "node1")

	newManager := func(globalExpect instance.MonitorGlobalExpect) *Manager {
		m := newTestManager(&pubSpy{})
		m.path = naming.Path{Namespace: "ns", Kind: naming.KindVol, Name: "nfs-data"}
		m.state.GlobalExpect = globalExpect
		m.state.Children = map[string]status.T{relation: status.Down}
		return m
	}
	setChild := func(p provisioned.T, globalExpect instance.MonitorGlobalExpect, state instance.MonitorState) {
		instance.StatusData.Set(child, "node1", &instance.Status{Avail: status.Down, Provisioned: p})
		instance.MonitorData.Set(child, "node1", &instance.Monitor{GlobalExpect: globalExpect, State: state})
	}

	setChild(provisioned.True, instance.MonitorGlobalExpectPurged, instance.MonitorStateUnprovisionProgress)
	assert.True(t, newManager(instance.MonitorGlobalExpectPurged).setWaitChildren(), "purge, child provisioned")
	assert.True(t, newManager(instance.MonitorGlobalExpectUnprovisioned).setWaitChildren(), "unprovision, child provisioned")
	assert.False(t, newManager(instance.MonitorGlobalExpectStarted).setWaitChildren(), "start, child down")

	setChild(provisioned.False, instance.MonitorGlobalExpectPurged, instance.MonitorStateUnprovisionSuccess)
	assert.False(t, newManager(instance.MonitorGlobalExpectPurged).setWaitChildren(), "child unprovisioned")

	setChild(provisioned.True, instance.MonitorGlobalExpectPurged, instance.MonitorStatePurgeFailed)
	assert.False(t, newManager(instance.MonitorGlobalExpectPurged).setWaitChildren(), "child purge failed")

	setChild(provisioned.True, instance.MonitorGlobalExpectNone, instance.MonitorStateStartFailure)
	assert.True(t, newManager(instance.MonitorGlobalExpectPurged).setWaitChildren(), "child not torn down")

	instance.StatusData.Unset(child, "node1")
	instance.MonitorData.Unset(child, "node1")
	assert.False(t, newManager(instance.MonitorGlobalExpectPurged).setWaitChildren(), "child deleted")
}
