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
