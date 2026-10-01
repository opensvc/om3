package resfszfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The copy on the destination is mounted as Start mounts the dataset: by its
// mountpoint property, or on the mount point of the resource for a legacy
// one, with its mount options.
func TestRemoteMountCmdline(t *testing.T) {
	r := &T{Device: "tank/vm1", MountPoint: "/srv/vm1"}
	assert.Equal(t, "/usr/sbin/zfs mount 'tank/vm1'", r.remoteMountCmdline(false))
	assert.Equal(t, "/usr/sbin/zfs umount 'tank/vm1'", r.remoteUmountCmdline(false))
	assert.Equal(t, "mkdir -p '/srv/vm1' && mount -t zfs 'tank/vm1' '/srv/vm1'", r.remoteMountCmdline(true))
	assert.Equal(t, "umount '/srv/vm1'", r.remoteUmountCmdline(true))

	r.MountOptions = "noatime"
	assert.Equal(t, "mkdir -p '/srv/vm1' && mount -t zfs -o 'noatime' 'tank/vm1' '/srv/vm1'", r.remoteMountCmdline(true))
}
