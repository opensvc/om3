//go:build linux

package poolshare

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/util/mountinfo"
)

// A share on a fuse filesystem is one the nodes share when the fuse
// filesystem is a network or cluster one, glusterfs or ceph, by the type of
// the mount holding it: the statfs type of fuse tells none of what it serves.
func TestCheckSharedFUSE(t *testing.T) {
	entries := []mountinfo.Entry{
		{Target: "/", FSType: "ext4"},
		{Target: "/srv/gluster", FSType: "fuse.glusterfs"},
		{Target: "/srv/ceph", FSType: "fuse.ceph-fuse"},
		{Target: "/srv/sshfs", FSType: "fuse.sshfs"},
		{Target: "/srv/gluster/local", FSType: "fuse.bindfs"},
		{Target: "/srv/stacked", FSType: "fuse.bindfs"},
		{Target: "/srv/stacked", FSType: "fuse.glusterfs"},
	}
	assert.NoError(t, checkSharedFUSE("/srv/gluster/pool", entries))
	assert.NoError(t, checkSharedFUSE("/srv/gluster", entries))
	assert.NoError(t, checkSharedFUSE("/srv/ceph/pool", entries))
	assert.NoError(t, checkSharedFUSE("/srv/stacked/pool", entries), "the last mount made over a mount point holds it")
	assert.ErrorContains(t, checkSharedFUSE("/srv/sshfs/pool", entries), "is on a fuse.sshfs filesystem, not one the nodes share")
	assert.ErrorContains(t, checkSharedFUSE("/srv/gluster/local/pool", entries), "fuse.bindfs", "the deepest mount holds it")
	assert.ErrorContains(t, checkSharedFUSE("/srv/glusterx/pool", entries), "ext4", "a mount point is a path, not a prefix")
}
