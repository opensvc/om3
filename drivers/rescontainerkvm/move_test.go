package rescontainerkvm

import (
	"strings"
	"testing"
	"time"

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
		migrateArgs("vm1", "qemu+ssh://n2/system", nil, true),
		"shared storage: nothing is mirrored, so writes have nothing to wait for")
	assert.Equal(t,
		[]string{"migrate", "--live", "--persistent", "--copy-storage-all", "--migrate-disks", "vda,vdb", "--copy-storage-synchronous-writes", "vm1", "qemu+ssh://n2/system"},
		migrateArgs("vm1", "qemu+ssh://n2/system", []string{"vda", "vdb"}, true))
	assert.Equal(t,
		[]string{"migrate", "--live", "--persistent", "--copy-storage-all", "--migrate-disks", "vda,vdb", "vm1", "qemu+ssh://n2/system"},
		migrateArgs("vm1", "qemu+ssh://n2/system", []string{"vda", "vdb"}, false),
		"a virsh older than libvirt 8.0 does not know the option")
}

// The option is looked for in what virsh says of its migrate command.
func TestHasSyncWritesOption(t *testing.T) {
	assert.True(t, hasSyncWritesOption("    --copy-storage-synchronous-writes  force guest disk writes to be synchronously written"))
	assert.False(t, hasSyncWritesOption("    --copy-storage-all  migration with non-shared storage with full disk copy"))
}

// A migration copying no disk is bounded by the stop_timeout of the guest, as
// before. One copying disks takes the time to copy them, and is bounded by the
// stop action only. migrate_timeout, when set, bounds both.
func TestMigrateTimeout(t *testing.T) {
	stop, migrate := 2*time.Minute, 30*time.Minute
	assert.Equal(t, stop, migrateTimeout(nil, &stop, false), "shared storage")
	assert.Equal(t, time.Duration(0), migrateTimeout(nil, &stop, true), "copied disks")
	assert.Equal(t, migrate, migrateTimeout(&migrate, &stop, false))
	assert.Equal(t, migrate, migrateTimeout(&migrate, &stop, true))
}
