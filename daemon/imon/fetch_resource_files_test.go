package imon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
)

// A resource file lives where the resource that holds it lives: the
// configuration of a container in the filesystem that carries the container.
// A node that does not run the object does not hold that filesystem, so there
// is nowhere to fetch into, and asking the peer for the file every time it
// refreshes its status asks for something that cannot land.
func TestAFileIsFetchedOnlyWhereItCanLand(t *testing.T) {
	held := t.TempDir()

	dir, ok := fetchDir(filepath.Join(held, "config"))
	assert.True(t, ok, "the directory is there, so the file has somewhere to go")
	assert.Equal(t, held, dir)

	dir, ok = fetchDir(filepath.Join(held, "base", "c12lxc2", "config"))
	assert.False(t, ok, "the filesystem carrying it is mounted where the object runs")
	assert.Equal(t, filepath.Join(held, "base", "c12lxc2"), dir)

	// A file whose directory is a file is not one to write into either.
	require.NoError(t, os.WriteFile(filepath.Join(held, "notadir"), []byte("x"), 0600))
	_, ok = fetchDir(filepath.Join(held, "notadir", "config"))
	assert.False(t, ok)
}

// The reason is said once: the status that triggers the fetch arrives every
// time the peer refreshes it, and the object runs there for as long as it
// runs there.
func TestTheReasonAFileIsNotFetchedIsSaidOnce(t *testing.T) {
	m := newFilesManager()
	assert.False(t, m.skipped["/srv/x/config"])
	m.skipped["/srv/x/config"] = true
	assert.True(t, m.skipped["/srv/x/config"], "said once, and not again")
	delete(m.skipped, "/srv/x/config")
	assert.False(t, m.skipped["/srv/x/config"], "and said again once it has been there and gone")
}

// The files of a resource are fetched from a peer only when they are the
// files of the same thing there: a container named vm1 on the peer and vm2
// here holds the configuration of vm1 there, which installed here would have
// the hypervisor define vm1 on this node. The resources whose files do not
// differ from node to node report no scope, and compare equal.
func TestFilesAreFetchedFromAPeerOfTheSameScopeOnly(t *testing.T) {
	assert.True(t, sameSyncScope(resource.Status{SyncScope: "vm1"}, resource.Status{SyncScope: "vm1"}), "the same container on both nodes")
	assert.False(t, sameSyncScope(resource.Status{SyncScope: "vm1"}, resource.Status{SyncScope: "vm2"}), "a container named per node")
	assert.True(t, sameSyncScope(resource.Status{}, resource.Status{}), "a resource with no scope")
}

// The reason the files of a resource are not fetched is said once, as the
// one of a file.
func TestTheReasonAScopeIsNotFetchedIsSaidOnce(t *testing.T) {
	m := &Manager{files: newFilesManager(), log: plog.NewDefaultLogger()}
	m.skipResourceScope("container#1", "vm1", "vm2")
	assert.True(t, m.files.skipped[scopeSkipKey("container#1")])
	assert.False(t, m.files.skipped["container#1"], "a file named as the rid is not marked")
}

// The files are fetched from the peer holding the files of the most of the
// same things the fetch takes: up there and down here. With a container named
// per node, a peer may run another one, whose files the fetch skips, and hold
// the files of the other resources all the same.
func TestTheFilesArePickedFromThePeerOfTheMostSameScopes(t *testing.T) {
	files := resource.Files{{Name: "/etc/libvirt/qemu/vm.xml"}}
	up, down := status.Up, status.Down
	local := instance.Status{Resources: instance.ResourceStatuses{
		"container#1": {Status: down, SyncScope: "vm1"},
		"disk#1":      {Status: down},
		"disk#2":      {Status: up},
	}}
	same := instance.Status{Resources: instance.ResourceStatuses{
		"container#1": {Status: up, SyncScope: "vm1", Files: files},
		"disk#1":      {Status: up, Files: files},
	}}
	partly := instance.Status{Resources: instance.ResourceStatuses{
		"container#1": {Status: up, SyncScope: "vm2", Files: files},
		"disk#1":      {Status: up, Files: files},
	}}
	other := instance.Status{Resources: instance.ResourceStatuses{
		"container#1": {Status: up, SyncScope: "vm2", Files: files},
	}}
	unfetchable := instance.Status{Resources: instance.ResourceStatuses{
		"container#1": {Status: down, SyncScope: "vm1", Files: files},
		"disk#2":      {Status: up, Files: files},
	}}
	assert.Equal(t, 2, syncScopeMatches(same, local, false))
	assert.Equal(t, 1, syncScopeMatches(partly, local, false), "the files of the disk are the same, the container ones are not")
	assert.Equal(t, 0, syncScopeMatches(other, local, false), "nothing of the peer is to fetch")
	assert.Equal(t, 0, syncScopeMatches(unfetchable, local, false), "a resource down there, or up here, is not fetched")
	assert.Equal(t, 1, syncScopeMatches(unfetchable, local, true), "a resource the placement stopped there is fetched")
}

// No file is fetched from a peer while the instances are torn down: the
// definition of a kvm domain the unprovision of a follower undefined came
// back from the leader, up until the followers are done, and stayed once the
// object was gone, refusing the provision of the next domain of the name.
func TestNoFileIsFetchedWhileTheInstancesAreTornDown(t *testing.T) {
	peerUp := &msgbus.InstanceStatusUpdated{Node: "node2", Value: instance.Status{Avail: status.Up}}
	m := &Manager{}
	m.state.LocalExpect = instance.MonitorLocalExpectNone
	assert.True(t, m.needFetchResourceFiles(peerUp))
	for _, ge := range []instance.MonitorGlobalExpect{
		instance.MonitorGlobalExpectPurged,
		instance.MonitorGlobalExpectUnprovisioned,
		instance.MonitorGlobalExpectDeleted,
	} {
		m.state.GlobalExpect = ge
		assert.False(t, m.needFetchResourceFiles(peerUp), "global expect %s", ge)
	}
	for _, ge := range []instance.MonitorGlobalExpect{
		instance.MonitorGlobalExpectNone,
		instance.MonitorGlobalExpectStarted,
		instance.MonitorGlobalExpectFrozen,
	} {
		m.state.GlobalExpect = ge
		assert.True(t, m.needFetchResourceFiles(peerUp), "global expect %s", ge)
	}
}

// The source of a placement stays the authority for its files once the
// orchestration stopped it: a file changed shortly before the move is listed
// first by the status the source refreshes at the end of its stop, when no
// instance is up to fetch it from. A peer down otherwise is not one: its
// files may be the ones of a definition left behind long ago.
func TestTheSourceAPlacementStoppedIsAnAuthorityForItsFiles(t *testing.T) {
	newManager := func() *Manager {
		m := newTestManager(&pubSpy{})
		m.state.GlobalExpect = instance.MonitorGlobalExpectPlacedAt
		m.state.GlobalExpectOptions = instance.MonitorGlobalExpectOptionsPlacedAt{Destination: []string{"node1"}}
		m.state.OrchestrationID = uuid.New()
		for _, nodename := range []string{"node2", "node3"} {
			m.instMonitor[nodename] = instance.Monitor{OrchestrationID: m.state.OrchestrationID}
			m.instStatus[nodename] = instance.Status{Avail: status.Down}
		}
		// node2 is seen up under the orchestration, before it stops.
		m.instStatus["node2"] = instance.Status{Avail: status.Up}
		m.recordFilesSource("node2")
		m.instStatus["node2"] = instance.Status{Avail: status.Down}
		return m
	}
	stopped := func(nodename string) *msgbus.InstanceStatusUpdated {
		return &msgbus.InstanceStatusUpdated{Node: nodename, Value: instance.Status{Avail: status.Down}}
	}

	m := newManager()
	assert.True(t, m.needFetchResourceFiles(stopped("node2")), "the source the orchestration stopped")
	assert.Equal(t, []string{"node2"}, m.stoppedSources())
	assert.False(t, m.needFetchResourceFiles(stopped("node3")), "a peer down before the orchestration")

	m = newManager()
	m.state.OrchestrationID = uuid.New()
	assert.False(t, m.needFetchResourceFiles(stopped("node2")), "the source of another orchestration")

	m = newManager()
	m.instMonitor["node2"] = instance.Monitor{}
	assert.False(t, m.needFetchResourceFiles(stopped("node2")), "a source out of the orchestration")

	m = newManager()
	m.state.GlobalExpectOptions = instance.MonitorGlobalExpectOptionsPlacedAt{Destination: []string{"node3"}}
	assert.False(t, m.needFetchResourceFiles(stopped("node2")), "a node the object is not placed on")

	m = newManager()
	m.state.GlobalExpect = instance.MonitorGlobalExpectStopped
	assert.False(t, m.needFetchResourceFiles(stopped("node2")), "a stop moves nothing")
}
