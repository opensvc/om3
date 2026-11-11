package driverdb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
)

// objectKinds are the object kinds whose keywords are grouped by the groups
// of the objects: the cluster configuration, a ccfg, has its own.
var objectKinds = []naming.Kind{
	naming.KindSvc,
	naming.KindVol,
	naming.KindNscfg,
	naming.KindSec,
	naming.KindUsr,
	naming.KindCfg,
}

// isDriverKeyword says a keyword of the store of a kind is the clone of a
// keyword of a driver manifest, which the store names by the group and the
// name of the driver.
func isDriverKeyword(kw *keywords.Keyword) bool {
	group := driver.NewGroup(kw.Section)
	if !group.IsResource() || len(kw.Types) != 1 {
		return false
	}
	return resource.NewResourceFunc(driver.NewID(group, kw.Types[0])) != nil
}

// objectKeywords returns the keywords of the store of a kind that are not the
// ones of a driver: the ones of the DEFAULT section, of the sections that are
// not resources, and the ones any section accepts.
func objectKeywords(kind naming.Kind) keywords.Store {
	l := make(keywords.Store, 0)
	for _, kw := range object.KeywordStoreWithDrivers(kind) {
		if !isDriverKeyword(kw) {
			l = append(l, kw)
		}
	}
	return l
}

// TestKeywordGroupsOfObjects verifies no keyword of an object kind, other
// than the ones of its drivers, is left in the group of the keywords not
// grouped yet, and that each names a group of the objects, which the api
// describes with no driver: a driver group of the same id, as timeouts, is
// not the one of the object keyword.
func TestKeywordGroupsOfObjects(t *testing.T) {
	ids := make(map[string]keywords.Group)
	for _, g := range object.ObjectKeywordGroups() {
		ids[g.ID] = g
	}
	for _, kind := range objectKinds {
		t.Run(kind.String(), func(t *testing.T) {
			store := objectKeywords(kind)
			require.NotEmpty(t, store)
			for _, kw := range store {
				id := kw.GroupID()
				if !assert.NotEqualf(t, keywords.GroupOtherID, id, "%s is not grouped", kwID(kw)) {
					continue
				}
				want, ok := ids[id]
				if !assert.Truef(t, ok, "%s: group %q is not a group of the objects", kwID(kw), id) {
					continue
				}
				got := object.KeywordGroups(keywords.Store{kw})
				assert.Equalf(t, []keywords.DriverGroup{{Group: want}}, got, "%s: the api describes the group of the objects", kwID(kw))
			}
		})
	}
}

// TestKeywordGroupsOfObjectsLeaveDriverGroups verifies a keyword of a driver
// naming a group of the id of a group of the objects, as timeouts, is still
// described as the group its driver declares.
func TestKeywordGroupsOfObjectsLeaveDriverGroups(t *testing.T) {
	for _, kind := range []naming.Kind{naming.KindSvc, naming.KindVol} {
		for _, kw := range object.KeywordStoreWithDrivers(kind) {
			if !isDriverKeyword(kw) {
				continue
			}
			if _, shared := manifest.SharedGroup(kw.GroupID()); shared {
				continue
			}
			for _, g := range object.KeywordGroups(keywords.Store{kw}) {
				assert.NotEmptyf(t, g.Driver, "%s: group %q is described with no driver", kwID(kw), g.ID)
			}
		}
	}
}

// TestKeywordGroupsOfObjectsInDriverListing verifies the keywords any section
// accepts, as pg_cpus, listed with the keywords of a driver as the api lists
// a driver, copied into its section, are described by the group of the
// objects they name, or by the group of the driver of that id.
func TestKeywordGroupsOfObjectsInDriverListing(t *testing.T) {
	store := object.KeywordStoreWithDrivers(naming.KindSvc)
	for drvID := range driverManifests() {
		l, err := store.DriverKeywords(drvID.Group.String(), drvID.Name, naming.KindSvc)
		if err != nil {
			continue
		}
		groups := object.KeywordGroups(l)
		for _, kw := range l {
			found := false
			for _, g := range groups {
				if g.ID == kw.GroupID() && (g.Driver == "" || g.Driver == drvID.String()) {
					found = true
					break
				}
			}
			assert.Truef(t, found, "%s: %s: group %q is not described", drvID, kw.Option, kw.GroupID())
		}
	}
}

// TestKeywordGroupsOfObjectsDeclaration verifies every group of the objects
// has a name and a text, an id of its own, and none of a group every
// resource has, which the api also describes with no driver.
func TestKeywordGroupsOfObjectsDeclaration(t *testing.T) {
	seen := make(map[string]bool)
	for _, g := range object.ObjectKeywordGroups() {
		assert.NotEmptyf(t, g.ID, "a group of the objects has no id")
		assert.NotEmptyf(t, g.Name, "group %s has no name", g.ID)
		assert.NotEmptyf(t, g.Text, "group %s has no text", g.ID)
		assert.Falsef(t, seen[g.ID], "group %s declared twice", g.ID)
		seen[g.ID] = true
		_, shared := manifest.SharedGroup(g.ID)
		assert.Falsef(t, shared, "group %s is one every resource has", g.ID)
	}
}

// TestKeywordGroupsOfObjectsPinned pins the groups of a few keywords of the
// objects.
func TestKeywordGroupsOfObjectsPinned(t *testing.T) {
	cases := []struct {
		kind    naming.Kind
		section string
		option  string
		group   string
	}{
		{naming.KindSvc, "DEFAULT", "nodes", "nodes"},
		{naming.KindSvc, "DEFAULT", "topology", "topology"},
		{naming.KindSvc, "DEFAULT", "flex_target", "topology"},
		{naming.KindSvc, "DEFAULT", "orchestrate", "orchestration"},
		{naming.KindSvc, "DEFAULT", "monitor_action", "ha"},
		{naming.KindSvc, "DEFAULT", "start_timeout", "timeouts"},
		{naming.KindSvc, "DEFAULT", "status_schedule", "scheduling"},
		{naming.KindSvc, "", "pg_cpus", "pg"},
		{naming.KindVol, "DEFAULT", "pool", "volume"},
		{naming.KindNscfg, "claim", "limit", "claim"},
		{naming.KindNscfg, "DEFAULT", "rootless_users", "rootless"},
		{naming.KindSec, "store", "url", "store"},
		{naming.KindSec, "acme", "webroot", "acme"},
		{naming.KindSec, "DEFAULT", "cn", "subject"},
		{naming.KindUsr, "DEFAULT", "grant", "grants"},
		{naming.KindCfg, "DEFAULT", "share", "sharing"},
	}
	for _, c := range cases {
		found := false
		for _, kw := range objectKeywords(c.kind) {
			if kw.Section != c.section || kw.Option != c.option {
				continue
			}
			found = true
			assert.Equalf(t, c.group, kw.Group, "%s %s", c.kind, kwID(kw))
		}
		assert.Truef(t, found, "%s %s.%s not found", c.kind, c.section, c.option)
	}
}
