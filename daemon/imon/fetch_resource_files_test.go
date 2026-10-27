package imon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
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

// The files are fetched from a peer holding the files of the same things:
// with a container named per node, the first up peer may run another one,
// whose files are all skipped, while another peer runs the same.
func TestTheFilesArePickedFromAPeerOfTheSameScopes(t *testing.T) {
	local := instance.Status{Resources: instance.ResourceStatuses{
		"container#1": {SyncScope: "vm1"},
		"fs#1":        {},
	}}
	files := resource.Files{{Name: "/etc/libvirt/qemu/vm.xml"}}
	same := instance.Status{Resources: instance.ResourceStatuses{
		"container#1": {SyncScope: "vm1", Files: files},
		"fs#1":        {},
	}}
	other := instance.Status{Resources: instance.ResourceStatuses{
		"container#1": {SyncScope: "vm2", Files: files},
	}}
	noFiles := instance.Status{Resources: instance.ResourceStatuses{
		"container#1": {SyncScope: "vm2"},
	}}
	assert.True(t, sameSyncScopes(same, local))
	assert.False(t, sameSyncScopes(other, local), "the peer holds the files of another container")
	assert.True(t, sameSyncScopes(noFiles, local), "a resource holding no file is no reason to pass a peer by")
}
