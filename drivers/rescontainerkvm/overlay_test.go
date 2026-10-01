package rescontainerkvm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each copied disk gets an overlay in the image directory of libvirt, outside
// the datasets a move sends, over the disk as the definition names it, of the
// format the definition says, raw when it says none.
func TestNewMoveOverlays(t *testing.T) {
	l := newMoveOverlays("vm1", []moveDisk{
		{Target: "vda", Source: "/dev/zvol/tank/vm1"},
		{Target: "vdb", Source: "/srv/vm1/data.qcow2", IsFile: true, Format: "qcow2"},
	})
	assert.Equal(t, []moveOverlay{
		{Target: "vda", Base: "/dev/zvol/tank/vm1", Format: "raw", Path: "/var/lib/libvirt/images/vm1.vda.osvc-move.qcow2"},
		{Target: "vdb", Base: "/srv/vm1/data.qcow2", Format: "qcow2", Path: "/var/lib/libvirt/images/vm1.vdb.osvc-move.qcow2"},
	}, l)
}

// The snapshot puts an overlay over the copied disks, and names every other
// disk as not snapshotted: one it did not name would be snapshotted the way
// its definition says. It keeps no snapshot metadata, and is all or nothing.
func TestSnapshotArgs(t *testing.T) {
	overlays := newMoveOverlays("vm1", []moveDisk{{Target: "vda", Source: "/dev/zvol/tank/vm1"}})
	assert.Equal(t, []string{
		"snapshot-create-as", "vm1", "--name", "osvc-move", "--disk-only", "--no-metadata", "--atomic",
		"--diskspec", "vda,snapshot=external,file=/var/lib/libvirt/images/vm1.vda.osvc-move.qcow2",
		"--diskspec", "sda,snapshot=no",
		"--diskspec", "vdc,snapshot=no",
	}, snapshotArgs("vm1", overlays, []string{"vda", "sda", "vdc"}))
}

// The overlay of the destination is over the copy of the disk the move sent
// there, at the same path, of the same format, and takes its size.
func TestOverlayCreateCmdline(t *testing.T) {
	o := newMoveOverlays("vm1", []moveDisk{{Target: "vdb", Source: "/srv/vm1/data.qcow2", Format: "qcow2"}})[0]
	assert.Equal(t,
		"qemu-img create -q -f qcow2 -b '/srv/vm1/data.qcow2' -F 'qcow2' '/var/lib/libvirt/images/vm1.vdb.osvc-move.qcow2'",
		overlayCreateCmdline(o))
}

func TestBlockcommitArgs(t *testing.T) {
	assert.Equal(t, []string{"blockcommit", "vm1", "vda", "--active", "--pivot", "--wait"}, blockcommitArgs("vm1", "vda"))
}

// Every disk is named in the snapshot, a disk with no source on the host
// among them.
func TestParseDiskTargets(t *testing.T) {
	l, err := parseDiskTargets(strings.NewReader(`<domain><devices>
  <disk type='block'><source dev='/dev/zvol/tank/vm1'/><target dev='vda'/></disk>
  <disk type='file' device='cdrom'><target dev='sda'/><readonly/></disk>
  <disk type='network'><source protocol='rbd' name='p/vm1'/><target dev='vdb'/></disk>
</devices></domain>`))
	require.NoError(t, err)
	assert.Equal(t, []string{"vda", "sda", "vdb"}, l)
}
