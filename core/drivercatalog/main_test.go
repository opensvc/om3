package drivercatalog_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/drivercatalog"

	_ "github.com/opensvc/om3/v3/core/driverdb"
)

func entries(t *testing.T) map[string]drivercatalog.Entry {
	t.Helper()
	m := make(map[string]drivercatalog.Entry)
	for _, e := range drivercatalog.List() {
		m[e.ID.String()] = e
	}
	require.NotEmpty(t, m, "the drivers must be registered")
	return m
}

// Every resource driver says what it does, in a line a picker shows under
// its name.
func TestEveryResourceDriverHasADoc(t *testing.T) {
	for name, e := range entries(t) {
		if !e.ID.Group.IsResource() {
			continue
		}
		assert.NotEmpty(t, e.Doc, "%s has no doc", name)
	}
}

// The features of a driver are words of the vocabulary, in its order.
func TestFeaturesAreOfTheVocabulary(t *testing.T) {
	rank := make(map[drivercatalog.Feature]int)
	for i, f := range drivercatalog.Features {
		require.NotEmpty(t, f.Doc, "%s has no doc", f.Name)
		rank[f.Name] = i
	}
	for name, e := range entries(t) {
		last := -1
		for _, f := range e.Features {
			i, ok := rank[f]
			require.True(t, ok, "%s: %s is no word of the vocabulary", name, f)
			assert.Greater(t, i, last, "%s: %s out of order", name, f)
			last = i
		}
	}
}

// A virtual driver says the drivers it selects among, in the order it
// prefers them, and an alias the driver it is another name of.
func TestVirtualAndAliasDrivers(t *testing.T) {
	m := entries(t)
	assert.Equal(t, driver.IDs{driver.NewID(driver.GroupContainer, "docker"), driver.NewID(driver.GroupContainer, "podman")}, m["container.oci"].Selects)
	assert.Equal(t, driver.IDs{driver.NewID(driver.GroupTask, "docker"), driver.NewID(driver.GroupTask, "podman")}, m["task.oci"].Selects)
	assert.Equal(t, driver.NewID(driver.GroupDisk, "vg"), m["disk.lvm"].AliasOf)
	assert.Equal(t, driver.NewID(driver.GroupIP, "netns"), m["ip.docker"].AliasOf)
	assert.True(t, m["disk.vg"].AliasOf.IsZero())
	assert.Empty(t, m["disk.vg"].Selects)
}

// A driver serving several types has a feature for the types having it
// only: fs.host makes, grows and has snapshotted an ext4, and only mounts a
// nfs.
func TestFeaturesOfTheTypesOfAMultiTypeDriver(t *testing.T) {
	m := entries(t)
	assert.Equal(t, []drivercatalog.Feature{"provision", "unprovision", "start", "stop", "snapshot", "resize"}, m["fs.ext4"].Features)
	assert.Equal(t, []drivercatalog.Feature{"start", "stop"}, m["fs.nfs"].Features)
	assert.Contains(t, m["task.host"].Features, drivercatalog.FeatureRun)
	assert.Contains(t, m["container.kvm"].Features, drivercatalog.FeatureMove)
	assert.Contains(t, m["sync.zfs"].Features, drivercatalog.FeatureSync)
}
