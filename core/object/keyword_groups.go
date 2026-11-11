package object

import (
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/resource"
)

// KeywordGroups returns the groups the keywords of the store belong to, each
// once per driver declaring it, in the order a wizard shows them.
//
// A group every resource has, the one of the keywords not grouped yet, and
// a group of the keywords of the objects, as the DEFAULT section holds them,
// are returned once, with no driver. A group a driver declares is returned
// with the driver of each keyword naming it: the drivers its section and
// types name, as the store of an object kind says them, or else the drivers
// holding the keyword, as a keyword looked up in a section of a
// configuration is the one of the driver manifest.
//
// A keyword of the node or cluster configuration names a group of these
// configurations, returned with no driver, or with the section type
// declaring one of its id, as array.hp3par.
func KeywordGroups(store keywords.Store) []keywords.DriverGroup {
	l := make([]keywords.DriverGroup, 0)
	seen := make(map[[2]string]bool)
	add := func(drv string, g keywords.Group) {
		id := [2]string{drv, g.ID}
		if seen[id] {
			return
		}
		seen[id] = true
		l = append(l, keywords.DriverGroup{Driver: drv, Group: g})
	}
	for _, kw := range store {
		if l, ok := nodeKeywordGroups(kw); ok {
			for _, dg := range l {
				add(dg.Driver, dg.Group)
			}
			continue
		}
		id := kw.GroupID()
		if g, ok := manifest.SharedGroup(id); ok {
			add("", g)
			continue
		}
		if g, ok := objectKeywordGroup(kw); ok {
			add("", g)
			continue
		}
		for _, did := range keywordDrivers(kw) {
			if m := driverManifest(did); m != nil {
				if g, ok := m.OwnGroup(id); ok {
					add(did.String(), g)
				}
			}
		}
	}
	keywords.SortDriverGroups(l)
	return l
}

// DriverKeywordGroups returns the groups the keywords of a resource of the
// driver may name: the ones the driver declares, and the ones every resource
// has. It is the referrer hook the validation of a configuration reads the
// strategies of a section by.
func (t *actor) DriverKeywordGroups(did driver.ID) []keywords.Group {
	m := driverManifest(did)
	if m == nil {
		return nil
	}
	return m.AllGroups()
}

// driverManifest returns the manifest of a resource driver, nil for a driver
// that is not one or not registered.
func driverManifest(did driver.ID) *manifest.T {
	factory := resource.NewResourceFunc(did)
	if factory == nil {
		return nil
	}
	return manifest.Get(factory())
}

// keywordDrivers returns the drivers a keyword of a store is the one of.
//
// A keyword of the store of an object kind names its driver by its section
// and types. One looked up in a section of a configuration is the keyword
// of the driver manifest, naming none, so the drivers are the ones whose
// manifest holds a keyword of its option in its group.
func keywordDrivers(kw *keywords.Keyword) driver.IDs {
	if group := driver.NewGroup(kw.Section); group != driver.GroupUnknown && len(kw.Types) > 0 {
		l := make(driver.IDs, 0, len(kw.Types))
		for _, typ := range kw.Types {
			l = append(l, driver.ID{Group: group, Name: typ})
		}
		return l
	}
	l := make(driver.IDs, 0)
	for _, did := range driver.List() {
		m := driverManifest(did)
		if m == nil {
			continue
		}
		for _, other := range m.Keywords() {
			if other.Option == kw.Option && other.GroupID() == kw.GroupID() {
				l = append(l, did)
				break
			}
		}
	}
	return l
}
