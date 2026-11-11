package driverdb_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
)

// driverManifests returns the manifests of the registered resource drivers,
// by driver.
func driverManifests() map[driver.ID]*manifest.T {
	m := make(map[driver.ID]*manifest.T)
	for _, drvID := range driver.List() {
		factory := resource.NewResourceFunc(drvID)
		if factory == nil {
			// node drivers have no factory, and thus no manifest
			continue
		}
		m[drvID] = manifest.Get(factory())
	}
	return m
}

// TestKeywordGroupExists verifies every keyword of the node, of the cluster,
// of every object kind and of every driver names a group the api describes:
// a group every resource has, the group of the keywords not grouped yet, or
// a group its driver declares.
func TestKeywordGroupExists(t *testing.T) {
	for name, store := range allKeywordStores(t) {
		t.Run(name, func(t *testing.T) {
			require.NotEmpty(t, store)
			for _, kw := range store {
				id := kw.GroupID()
				found := false
				for _, g := range object.KeywordGroups(keywords.Store{kw}) {
					if g.ID == id {
						found = true
						break
					}
				}
				assert.Truef(t, found, "%s: group %q is declared nowhere", kwID(kw), id)
			}
		})
	}
}

// TestKeywordGroupOfDriverExists verifies the keywords of a driver manifest
// name a group the driver declares or every resource has, the manifest being
// where the validation reads the strategies of a section from.
func TestKeywordGroupOfDriverExists(t *testing.T) {
	for drvID, m := range driverManifests() {
		ids := make([]string, 0)
		for _, g := range m.AllGroups() {
			ids = append(ids, g.ID)
		}
		for _, kw := range m.Keywords() {
			assert.Containsf(t, ids, kw.GroupID(), "%s.%s: group %q is not one of the driver", drvID, kw.Option, kw.GroupID())
		}
	}
}

// TestKeywordGroupIDIsUnique verifies a driver declares no group of the id of
// a group every resource has, which would make one of the two unreachable,
// nor twice the same.
func TestKeywordGroupIDIsUnique(t *testing.T) {
	for drvID, m := range driverManifests() {
		seen := make(map[string]bool)
		for _, g := range m.Groups {
			_, shared := manifest.SharedGroup(g.ID)
			assert.Falsef(t, shared, "%s: group %q is one every resource has", drvID, g.ID)
			assert.Falsef(t, seen[g.ID], "%s: group %q declared twice", drvID, g.ID)
			seen[g.ID] = true
		}
	}
}

// TestKeywordStrategyOptionsExist verifies every option a strategy uses,
// excludes or is chosen by is the option of a keyword of the section of the
// driver of its group, for every driver whose keywords name the group.
func TestKeywordStrategyOptionsExist(t *testing.T) {
	for drvID, m := range driverManifests() {
		options := make([]string, 0)
		named := make(map[string]bool)
		for _, kw := range m.Keywords() {
			options = append(options, kw.Option)
			named[kw.GroupID()] = true
		}
		// The keywords every resource section holds, which a selector
		// may be, beside the ones of the driver.
		options = append(options, "type")
		for _, g := range m.AllGroups() {
			if !named[g.ID] && !slices.ContainsFunc(m.Groups, func(o keywords.Group) bool { return o.ID == g.ID }) {
				continue
			}
			for _, s := range g.Strategies {
				for _, option := range slices.Concat(s.Uses, s.Excludes, s.Requires) {
					assert.Containsf(t, options, option, "%s: group %s strategy %s names %q, no keyword of the driver", drvID, g.ID, s.ID, option)
				}
				for _, kop := range s.When {
					if kop.Key.Section != "" && kop.Key.Section != "DEFAULT" {
						continue
					}
					assert.Containsf(t, options, kop.Key.Option, "%s: group %s strategy %s is chosen by %q, no keyword of the driver", drvID, g.ID, s.ID, kop.Key.Option)
				}
			}
		}
	}
}

// TestKeywordGroupHasNameAndText verifies every group and every strategy has
// a name and a text: the text is read from a file embedded, and a missing
// file reads as an empty text.
func TestKeywordGroupHasNameAndText(t *testing.T) {
	check := func(owner string, g keywords.Group) {
		assert.NotEmptyf(t, g.ID, "%s: a group with no id", owner)
		assert.NotEmptyf(t, g.Name, "%s: group %s has no name", owner, g.ID)
		assert.NotEmptyf(t, g.Text, "%s: group %s has no text", owner, g.ID)
		for _, s := range g.Strategies {
			assert.NotEmptyf(t, s.ID, "%s: group %s has a strategy with no id", owner, g.ID)
			assert.NotEmptyf(t, s.Name, "%s: group %s strategy %s has no name", owner, g.ID, s.ID)
			assert.NotEmptyf(t, s.Text, "%s: group %s strategy %s has no text", owner, g.ID, s.ID)
			assert.NotEmptyf(t, s.Uses, "%s: group %s strategy %s uses no keyword", owner, g.ID, s.ID)
		}
	}
	check("every resource", keywords.GroupOther)
	for _, g := range manifest.SharedGroups {
		check("every resource", g)
	}
	for drvID, m := range driverManifests() {
		for _, g := range m.Groups {
			check(drvID.String(), g)
		}
	}
}

// TestKeywordGroupsOfIPDrivers pins the addressing group of the ip drivers
// plumbing an address, and its two strategies.
func TestKeywordGroupsOfIPDrivers(t *testing.T) {
	for _, name := range []string{"host", "netns"} {
		drvID := driver.NewID(driver.GroupIP, name)
		factory := resource.NewResourceFunc(drvID)
		if factory == nil {
			continue
		}
		m := manifest.Get(factory())
		g, ok := m.OwnGroup("addressing")
		require.Truef(t, ok, "%s declares the addressing group", drvID)
		ids := make([]string, 0)
		for _, s := range g.Strategies {
			ids = append(ids, s.ID)
		}
		assert.Equal(t, []string{"static", "ipam"}, ids)
		for _, kw := range m.Keywords() {
			switch kw.Option {
			case "name", "netmask", "network", "addr":
				assert.Equalf(t, "addressing", kw.Group, "%s.%s", drvID, kw.Option)
			}
		}
	}
}

// TestKeywordGroupsOfContainerDrivers verifies no keyword of a container
// driver is left in the group of the keywords not grouped yet, and pins the
// install group of the kvm driver and its strategies.
func TestKeywordGroupsOfContainerDrivers(t *testing.T) {
	n := 0
	for drvID, m := range driverManifests() {
		if drvID.Group != driver.GroupContainer {
			continue
		}
		n++
		for _, kw := range m.Keywords() {
			assert.NotEqualf(t, keywords.GroupOtherID, kw.GroupID(), "%s.%s is not grouped", drvID, kw.Option)
		}
	}
	if n == 0 {
		t.Skip("no container driver on this os")
	}
	factory := resource.NewResourceFunc(driver.NewID(driver.GroupContainer, "kvm"))
	require.NotNil(t, factory)
	g, ok := manifest.Get(factory()).OwnGroup("install")
	require.True(t, ok, "kvm declares the install group")
	ids := make([]string, 0)
	for _, s := range g.Strategies {
		ids = append(ids, s.ID)
	}
	assert.Equal(t, []string{"image", "cdrom", "virtinst"}, ids)
}
