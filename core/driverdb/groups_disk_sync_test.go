package driverdb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
)

// TestKeywordGroupsOfDiskAndSyncDrivers verifies no keyword of a disk or a
// sync driver is left in the group of the keywords not grouped yet, and that
// the drivers of a family declaring a group of the same id declare the same
// group, the one their family package holds.
func TestKeywordGroupsOfDiskAndSyncDrivers(t *testing.T) {
	for _, family := range []driver.Group{driver.GroupDisk, driver.GroupSync} {
		t.Run(family.String(), func(t *testing.T) {
			n := 0
			seen := make(map[string]keywords.Group)
			owner := make(map[string]driver.ID)
			for drvID, m := range driverManifests() {
				if drvID.Group != family {
					continue
				}
				n++
				for _, kw := range m.Keywords() {
					assert.NotEqualf(t, keywords.GroupOtherID, kw.GroupID(), "%s.%s is not grouped", drvID, kw.Option)
				}
				for _, g := range m.Groups {
					if other, ok := seen[g.ID]; ok {
						assert.Equalf(t, other, g, "%s and %s declare the same %s group", owner[g.ID], drvID, g.ID)
						continue
					}
					seen[g.ID] = g
					owner[g.ID] = drvID
				}
			}
			if n == 0 {
				t.Skipf("no %s driver on this os", family)
			}
		})
	}
}
