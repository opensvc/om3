package object

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

func (t *Node) lastRunFile(action, rid, base string) string {
	base = "last_" + base
	if rid != "" {
		base = base + "_" + rid
	}
	return base
}

func (t *Node) newScheduleEntry(action, section, rid, base string) schedule.Entry {
	k := key.T{Section: section, Option: "schedule"}
	def, err := t.MergedConfig().GetStringStrict(k)
	if err != nil {
		panic(err)
	}
	entry := schedule.Entry{
		Config: schedule.Config{
			Action:           action,
			Key:              k.String(),
			MaxParallel:      1,
			RequireCollector: true,
			Schedule:         def,
			StatefileKey:     t.lastRunFile(action, rid, base),
		},
		Node: hostname.Hostname(),
	}
	entry.LastRunAt = entry.LoadLast()
	return entry
}

// sectionScheduleActions is the scheduler action each section of the node
// configuration is scheduled through.
//
// The action is fixed per driver group, where it used to be "push" followed by
// the type of the section. One om command pushes every type of a group, "om
// node push array" pushing a pure array and an hds array alike, so a name
// carrying the type named a command nobody implements. It also made the entry
// unrunnable: daemon/scheduler turns an action into an argv through a fixed
// table, which a name built from configuration can never be in.
var sectionScheduleActions = map[driver.Group]string{
	driver.GroupArray:  "pusharray",
	driver.GroupBackup: "pushbackup",
	driver.GroupSwitch: "pushswitch",
}

func (t *Node) Schedules() schedule.Table {
	table := schedule.NewTable(
		t.newScheduleEntry("pushasset", "asset", "", "asset_push"),
		t.newScheduleEntry("checks", "checks", "", "checks_push"),
		t.newScheduleEntry("compliance_auto", "compliance", "", "comp_check"),
		t.newScheduleEntry("pushdisks", "disks", "", "disks_push"),
		t.newScheduleEntry("pushpkg", "packages", "", "packages_push"),
		t.newScheduleEntry("sysreport", "sysreport", "", "sysreport_push"),
	)
	// The merged configuration and not the node one: an array, a switch and a
	// backup are declared in cluster.conf, being reachable from every node
	// rather than belonging to one, and the node configuration holds none of
	// them. Reading only the node file found no section at all, so a schedule
	// written for an array was never a schedule entry.
	for _, s := range t.MergedConfig().SectionStrings() {
		rid, err := resourceid.Parse(s)
		if err != nil {
			continue
		}
		action, ok := sectionScheduleActions[rid.DriverGroup()]
		if !ok {
			// no schedule
			continue
		}
		drvType := t.MergedConfig().GetString(key.T{Section: s, Option: "type"})
		base := fmt.Sprintf("%s_%s_push", s, drvType)
		e := t.newScheduleEntry(action, s, rid.String(), base)
		table = table.Add(e)
	}
	return table
}
