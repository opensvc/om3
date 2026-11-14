package imon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
)

func newResourceEventTestManager() *Manager {
	m := &Manager{
		localhost: "n1",
		log:       plog.NewDefaultLogger(),
		instStatus: map[string]instance.Status{"n1": {Resources: instance.ResourceStatuses{
			"container#1": resource.Status{Type: "container.kvm", Info: map[string]any{"name": "vm1"}},
			"disk#1":      resource.Status{Type: "disk.drbd", Info: map[string]any{"res": "r1"}},
			"container#2": resource.Status{Type: "container.podman", Info: map[string]any{"name": "c1"}},
		}}},
		eventRefreshTimer: time.NewTimer(time.Hour),
	}
	m.eventRefreshTimer.Stop()
	return m
}

// A libvirt domain or a drbd resource changing on the node refreshes the
// status of the instance holding it, and of no other.
func TestAResourceEventRefreshesTheInstanceHoldingIt(t *testing.T) {
	m := newResourceEventTestManager()
	m.onVirtDomainUpdated(&msgbus.VirtDomainUpdated{Node: "n1", Name: "vm2"})
	m.onDrbdResourceUpdated(&msgbus.DrbdResourceUpdated{Node: "n1", Res: "r2"})
	m.onVirtDomainUpdated(&msgbus.VirtDomainUpdated{Node: "n2", Name: "vm1"})
	assert.False(t, m.eventRefreshScheduled, "no resource of the instance, or another node")

	m.onVirtDomainUpdated(&msgbus.VirtDomainUpdated{Node: "n1", Name: "vm1"})
	assert.True(t, m.eventRefreshScheduled)

	m = newResourceEventTestManager()
	m.onDrbdResourceUpdated(&msgbus.DrbdResourceUpdated{Node: "n1", Res: "r1"})
	assert.True(t, m.eventRefreshScheduled)

	// A name matching the info of another driver type is not one.
	m = newResourceEventTestManager()
	m.onVirtDomainUpdated(&msgbus.VirtDomainUpdated{Node: "n1", Name: "r1"})
	assert.False(t, m.eventRefreshScheduled)
}

// An event during an action of the object, here or on a peer, is likely its
// own doing: it refreshes nothing, the action evaluating the status it ends
// on.
func TestAResourceEventDuringAnActionRefreshesNothing(t *testing.T) {
	m := newResourceEventTestManager()
	m.instMonitor = map[string]instance.Monitor{"n2": {State: instance.MonitorStateStopProgress}}
	m.onVirtDomainUpdated(&msgbus.VirtDomainUpdated{Node: "n1", Name: "vm1"})
	assert.False(t, m.eventRefreshScheduled)
}

// A podman or docker container changing on the node refreshes the status of
// the instance holding it, a container of the same name of the other engine
// none.
func TestAContainerEventRefreshesTheInstanceHoldingIt(t *testing.T) {
	m := newResourceEventTestManager()
	m.onContainerUpdated(&msgbus.ContainerUpdated{Node: "n1", Type: "container.docker", Name: "c1"})
	assert.False(t, m.eventRefreshScheduled, "another engine")
	m.onContainerUpdated(&msgbus.ContainerUpdated{Node: "n1", Type: "container.podman", Name: "c1"})
	assert.True(t, m.eventRefreshScheduled)
}

// An md array changing on the node refreshes the status of the instance
// whose md resource names it, and of no other. An event naming no array
// refreshes the instances whose md resource status tells a warning, as a
// resync it may have caught, and not the clean ones.
func TestAnMdEventRefreshesTheInstanceHoldingTheArray(t *testing.T) {
	m := newResourceEventTestManager()
	m.onMdArrayUpdated(&msgbus.MdArrayUpdated{Node: "n1", Name: "md127"})
	m.onMdArrayUpdated(&msgbus.MdArrayUpdated{Node: "n1"})
	assert.False(t, m.eventRefreshScheduled, "no md resource")

	m.instStatus["n1"].Resources["disk#2"] = resource.Status{Type: "disk.md", Info: map[string]any{"dev": "md127"}}
	m.onMdArrayUpdated(&msgbus.MdArrayUpdated{Node: "n2", Name: "md127"})
	assert.False(t, m.eventRefreshScheduled, "another node")
	m.onMdArrayUpdated(&msgbus.MdArrayUpdated{Node: "n1", Name: "md126"})
	assert.False(t, m.eventRefreshScheduled, "another array")
	m.onMdArrayUpdated(&msgbus.MdArrayUpdated{Node: "n1"})
	assert.False(t, m.eventRefreshScheduled, "no array named, a clean md resource")
	m.onMdArrayUpdated(&msgbus.MdArrayUpdated{Node: "n1", Name: "md127"})
	assert.True(t, m.eventRefreshScheduled)

	m = newResourceEventTestManager()
	m.instStatus["n1"].Resources["disk#2"] = resource.Status{
		Type: "disk.md",
		Info: map[string]any{"dev": "md127"},
		Log:  []resource.StatusLogEntry{{Level: resource.WarnLevel, Message: "clean, resyncing"}},
	}
	m.onMdArrayUpdated(&msgbus.MdArrayUpdated{Node: "n1"})
	assert.True(t, m.eventRefreshScheduled, "no array named, an md resource resyncing")

	m = newResourceEventTestManager()
	m.instStatus["n1"].Resources["disk#2"] = resource.Status{Type: "disk.md"}
	m.onMdArrayUpdated(&msgbus.MdArrayUpdated{Node: "n1", Name: "md126"})
	assert.False(t, m.eventRefreshScheduled, "an md resource naming no array takes no named change")

	// An array assembled outside of the agent, for an md resource down, is
	// matched by its uuid.
	m = newResourceEventTestManager()
	m.instStatus["n1"].Resources["disk#2"] = resource.Status{Type: "disk.md", Info: map[string]any{"uuid": "a:b:c:d"}}
	m.onMdArrayUpdated(&msgbus.MdArrayUpdated{Node: "n1", Name: "md125", UUID: "e:f:g:h"})
	assert.False(t, m.eventRefreshScheduled, "another uuid")
	m.onMdArrayUpdated(&msgbus.MdArrayUpdated{Node: "n1", Name: "md125", UUID: "a:b:c:d"})
	assert.True(t, m.eventRefreshScheduled)
}

// An md event during an action of the object, here or on a peer, is not
// dropped: it is often the end of a resync the action started, while a peer
// still provisions. Its refresh is deferred until no action runs.
func TestAnMdEventDuringAnActionIsDeferred(t *testing.T) {
	m := newResourceEventTestManager()
	m.instStatus["n1"].Resources["disk#2"] = resource.Status{Type: "disk.md", Info: map[string]any{"dev": "md127"}}
	m.instMonitor = map[string]instance.Monitor{"n2": {State: instance.MonitorStateProvisionProgress}}
	m.onMdArrayUpdated(&msgbus.MdArrayUpdated{Node: "n1", Name: "md127"})
	assert.True(t, m.eventRefreshScheduled)
	m.onEventRefreshTimer()
	assert.True(t, m.eventRefreshDeferred, "deferred until the peer is done")
}
