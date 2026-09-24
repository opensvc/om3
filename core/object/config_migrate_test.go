package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

func migrationOf(t *testing.T, config string) Migration {
	t.Helper()
	p, err := naming.ParsePath("test/svc/foo")
	require.NoError(t, err)
	o, err := NewSvc(p, WithConfigData([]byte(config)), WithVolatile(true))
	require.NoError(t, err)
	return MigrateConfig(o.config)
}

func setOf(m Migration, k string) (string, bool) {
	for _, op := range m.Sets {
		if op.Key.String() == k {
			return op.Value, true
		}
	}
	return "", false
}

func unset(m Migration, k string) bool {
	for _, key := range m.Unsets {
		if key.String() == k {
			return true
		}
	}
	return false
}

// The filesystem driver of om2 made the volume it mounted. om3 describes that
// volume as a resource of its own, which is what makes it something the rest
// of om can see.
func TestAFilesystemThatMadeItsVolumeBecomesTwoResources(t *testing.T) {
	m := migrationOf(t, `
[fs#2]
type = ext4
dev = /dev/testvg/data
mnt = /srv/data
vg = testvg
size = 20g
create_options = -m 1
`)
	assert.Equal(t, "lv", must(t, m, "disk#2.type"))
	assert.Equal(t, "data", must(t, m, "disk#2.name"), "the volume is the last element of the device path")
	assert.Equal(t, "testvg", must(t, m, "disk#2.vg"))
	assert.Equal(t, "20g", must(t, m, "disk#2.size"))
	assert.Equal(t, "-m 1", must(t, m, "disk#2.create_options"),
		"om2 passed these to the volume create command, not to mkfs")
	assert.Equal(t, "{disk#2.exposed_devs[0]}", must(t, m, "fs#2.dev"))

	for _, k := range []string{"fs#2.vg", "fs#2.size", "fs#2.create_options"} {
		assert.Truef(t, unset(m, k), "%s is left behind", k)
	}
	assert.False(t, unset(m, "fs#2.mnt"), "what the filesystem still reads stays")
}

// A share of a volume group is a size om cannot count. Where the group is a
// resource of the object, it becomes arithmetic over what that resource
// reports, which om resolves itself and can grow.
func TestAShareOfAGroupOmHoldsBecomesArithmetic(t *testing.T) {
	const config = `
[disk#1]
type = vg
name = testvg
pvs = /dev/sdb

[fs#2]
type = ext4
dev = /dev/testvg/data
mnt = /srv/data
vg = testvg
size = 60%FREE

[fs#3]
type = xfs
dev = /dev/testvg/logs
mnt = /srv/logs
vg = testvg
size = 100%VG
`
	m := migrationOf(t, config)
	assert.Equal(t, "$(60% * {disk#1.free})", must(t, m, "disk#2.size"))
	assert.Equal(t, "$(100% * {disk#1.capacity})", must(t, m, "disk#3.size"),
		"a share of the whole group is a share of what it holds")
	assert.Equal(t, "testvg", must(t, m, "disk#3.vg"))
	assert.Empty(t, m.Notes[0:0])
}

// Where om holds no resource for the group, the share is carried over as it
// stands: it still makes the volume it always made, and the configuration
// says so.
func TestAShareOfAGroupOmDoesNotHoldIsKept(t *testing.T) {
	m := migrationOf(t, `
[fs#2]
type = ext4
dev = /dev/testvg/data
mnt = /srv/data
vg = testvg
size = 60%FREE
`)
	assert.Equal(t, "60%FREE", must(t, m, "disk#2.size"))
	assert.Contains(t, m.Notes[0], "om does not hold")
}

// A keyword written for one node moves with the node it was written for.
func TestAScopedSizeMovesWithItsScope(t *testing.T) {
	m := migrationOf(t, `
[fs#2]
type = ext4
dev = /dev/testvg/data
mnt = /srv/data
vg = testvg
size@node1 = 10g
size@node2 = 20g
`)
	assert.Equal(t, "10g", must(t, m, "disk#2.size@node1"))
	assert.Equal(t, "20g", must(t, m, "disk#2.size@node2"))
	assert.True(t, unset(m, "fs#2.size@node1"))
	assert.True(t, unset(m, "fs#2.size@node2"))
}

// A configuration keeps what nothing can migrate for it, and hears why: a
// volume group named for one node only says nothing about the others.
func TestWhatCannotBeMigratedIsSaidAndLeftAlone(t *testing.T) {
	m := migrationOf(t, `
[fs#2]
type = ext4
dev = /dev/mapper/mpatha
mnt = /srv/data
vg@node1 = testvg
size = 20g
`)
	assert.Empty(t, m.Sets)
	assert.Empty(t, m.Unsets)
	require.Len(t, m.Refusals, 1)
	assert.Contains(t, m.Refusals[0], "neither fs#2.vg nor fs#2.dev says which volume group")
}

// A size with no volume group to carve it from made no volume in om2, which
// only made one when a volume group was named.
func TestASizeWithNoVolumeGroupMadeNoVolume(t *testing.T) {
	m := migrationOf(t, `
[fs#2]
type = ext4
dev = /dev/testvg/data
mnt = /srv/data
size = 20g
`)
	assert.Empty(t, m.Sets)
	assert.Empty(t, m.Unsets)
	assert.Empty(t, m.Refusals)
}

// A filesystem that never made a volume has nothing to migrate.
func TestAFilesystemThatMadeNoVolumeIsLeftAlone(t *testing.T) {
	m := migrationOf(t, `
[fs#2]
type = ext4
dev = /dev/testvg/data
mnt = /srv/data
`)
	assert.Empty(t, m.Sets)
	assert.Empty(t, m.Unsets)
	assert.Empty(t, m.Refusals)
}

// The resource index of the filesystem is used for the volume, so that what
// belongs together reads together, unless something else has it.
func TestTheVolumeTakesTheIndexOfTheFilesystem(t *testing.T) {
	m := migrationOf(t, `
[disk#2]
type = loop
file = /var/lib/opensvc/pool/loop1/foo.img
size = 1g

[fs#2]
type = ext4
dev = /dev/testvg/data
mnt = /srv/data
vg = testvg
size = 20g
`)
	_, ok := setOf(m, "disk#fs2.type")
	assert.True(t, ok, "the index is taken, so the volume is named after the filesystem")
	assert.Equal(t, "{disk#fs2.exposed_devs[0]}", must(t, m, "fs#2.dev"))
}

func must(t *testing.T, m Migration, k string) string {
	t.Helper()
	v, ok := setOf(m, k)
	require.Truef(t, ok, "%s is not set", k)
	return v
}
