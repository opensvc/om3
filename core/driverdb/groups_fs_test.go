package driverdb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/resource"
)

// TestKeywordGroupsOfStorageDrivers verifies no keyword of a filesystem,
// volume or share driver is left in the group of the keywords not grouped
// yet, and that the filesystems sharing the keywords of the ones mounted
// declare the same groups for them.
func TestKeywordGroupsOfStorageDrivers(t *testing.T) {
	n := 0
	for drvID, m := range driverManifests() {
		switch drvID.Group {
		case driver.GroupFS, driver.GroupVolume, driver.GroupShare:
		default:
			continue
		}
		n++
		for _, kw := range m.Keywords() {
			assert.NotEqualf(t, keywords.GroupOtherID, kw.GroupID(), "%s.%s is not grouped", drvID, kw.Option)
		}
	}
	if n == 0 {
		t.Skip("no filesystem, volume or share driver on this os")
	}
	ext4 := resource.NewResourceFunc(driver.NewID(driver.GroupFS, "ext4"))
	if ext4 == nil {
		return
	}
	host := manifest.Get(ext4())
	for _, name := range []string{"zfs", "sgcp_nfs", "tmpfs", "btrfs"} {
		factory := resource.NewResourceFunc(driver.NewID(driver.GroupFS, name))
		if factory == nil {
			continue
		}
		m := manifest.Get(factory())
		for _, id := range []string{"device", "mount", "status", "permissions", "install"} {
			hg, ok := host.OwnGroup(id)
			require.Truef(t, ok, "fs.ext4 declares the %s group", id)
			g, ok := m.OwnGroup(id)
			require.Truef(t, ok, "fs.%s declares the %s group", name, id)
			assert.Equalf(t, hg, g, "fs.ext4 and fs.%s declare the same %s group", name, id)
		}
	}
}
