package rescontainerkvm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The disks of a domain are read with their target and their source, a
// device or a file. A disk with no source on the host is on no resource, and
// a read-only or shared one is not mirrored.
func TestParseMoveDisks(t *testing.T) {
	doc := `<domain type='kvm'>
  <name>vm1</name>
  <devices>
    <disk type='block' device='disk'>
      <source dev='/dev/zvol/tank/vm1'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <disk type='file' device='disk'>
      <source file='/srv/vm1/data.qcow2'/>
      <target dev='vdb' bus='virtio'/>
    </disk>
    <disk type='file' device='cdrom'>
      <source file='/srv/vm1/install.iso'/>
      <target dev='sda' bus='sata'/>
      <readonly/>
    </disk>
    <disk type='block' device='disk'>
      <source dev='/dev/drbd1'/>
      <target dev='vdc' bus='virtio'/>
      <shareable/>
    </disk>
    <disk type='file' device='cdrom'>
      <target dev='sdb' bus='sata'/>
      <readonly/>
    </disk>
    <disk type='network' device='disk'>
      <source protocol='rbd' name='pool/vm1'/>
      <target dev='vdd' bus='virtio'/>
    </disk>
  </devices>
</domain>`
	l, err := parseMoveDisks(strings.NewReader(doc))
	require.NoError(t, err)
	assert.Equal(t, []moveDisk{
		{Target: "vda", Source: "/dev/zvol/tank/vm1", Copyable: true},
		{Target: "vdb", Source: "/srv/vm1/data.qcow2", IsFile: true, Copyable: true},
		{Target: "sda", Source: "/srv/vm1/install.iso", IsFile: true, Copyable: false},
		{Target: "vdc", Source: "/dev/drbd1", Copyable: false},
	}, l)
}

// A migration mirrors the disks it is told to and only those, and mirrors
// nothing when the storage is shared.
func TestMigrateArgs(t *testing.T) {
	assert.Equal(t,
		[]string{"migrate", "--live", "--persistent", "vm1", "qemu+ssh://n2/system"},
		migrateArgs("vm1", "qemu+ssh://n2/system", nil))
	assert.Equal(t,
		[]string{"migrate", "--live", "--persistent", "--copy-storage-all", "--migrate-disks", "vda,vdb", "vm1", "qemu+ssh://n2/system"},
		migrateArgs("vm1", "qemu+ssh://n2/system", []string{"vda", "vdb"}))
}
