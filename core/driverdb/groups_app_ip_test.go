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

// TestKeywordGroupsOfAppAndIPDrivers verifies no keyword of an app or an ip
// driver is left in the group of the keywords not grouped yet.
func TestKeywordGroupsOfAppAndIPDrivers(t *testing.T) {
	n := 0
	for drvID, m := range driverManifests() {
		if drvID.Group != driver.GroupApp && drvID.Group != driver.GroupIP {
			continue
		}
		n++
		for _, kw := range m.Keywords() {
			assert.NotEqualf(t, keywords.GroupOtherID, kw.GroupID(), "%s.%s is not grouped", drvID, kw.Option)
		}
	}
	if n == 0 {
		t.Skip("no app or ip driver on this os")
	}
}

// TestKeywordGroupsOfHostTaskAreTheAppOnes verifies a host task, borrowing
// keywords of the app drivers, declares the environment and limits groups of
// the app drivers, and a process group of the id, name and order of theirs,
// so a reader sees one vocabulary.
func TestKeywordGroupsOfHostTaskAreTheAppOnes(t *testing.T) {
	taskFactory := resource.NewResourceFunc(driver.NewID(driver.GroupTask, "host"))
	appFactory := resource.NewResourceFunc(driver.NewID(driver.GroupApp, "simple"))
	if taskFactory == nil || appFactory == nil {
		t.Skip("no host task or simple app driver on this os")
	}
	task := manifest.Get(taskFactory())
	app := manifest.Get(appFactory())
	for _, id := range []string{"environment", "limits"} {
		tg, ok := task.OwnGroup(id)
		require.Truef(t, ok, "task.host declares the %s group", id)
		ag, ok := app.OwnGroup(id)
		require.Truef(t, ok, "app.simple declares the %s group", id)
		assert.Equalf(t, ag, tg, "task.host and app.simple declare the same %s group", id)
	}
	for _, id := range []string{"process", "status", "timeouts"} {
		tg, ok := task.OwnGroup(id)
		require.Truef(t, ok, "task.host declares the %s group", id)
		ag, ok := app.OwnGroup(id)
		require.Truef(t, ok, "app.simple declares the %s group", id)
		assert.Equalf(t, ag.Name, tg.Name, "task.host and app.simple name the %s group alike", id)
		assert.Equalf(t, ag.Order, tg.Order, "task.host and app.simple order the %s group alike", id)
	}
}
