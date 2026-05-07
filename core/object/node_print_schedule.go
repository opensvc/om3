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
	def, err := t.config.GetStringStrict(k)
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

func (t *Node) Schedules() schedule.Table {
	table := schedule.NewTable(
		t.newScheduleEntry("pushasset", "asset", "", "asset_push"),
		t.newScheduleEntry("checks", "checks", "", "checks_push"),
		t.newScheduleEntry("compliance_auto", "compliance", "", "comp_check"),
		t.newScheduleEntry("pushdisks", "disks", "", "disks_push"),
		t.newScheduleEntry("pushpkg", "packages", "", "packages_push"),
		t.newScheduleEntry("pushpatch", "patches", "", "patches_push"),
		t.newScheduleEntry("sysreport", "sysreport", "", "sysreport_push"),
	)
	for _, s := range t.config.SectionStrings() {
		rid, err := resourceid.Parse(s)
		if err != nil {
			continue
		}
		switch rid.DriverGroup() {
		case driver.GroupArray:
		case driver.GroupSwitch:
		case driver.GroupBackup:
		default:
			// no schedule
			continue
		}
		drvType := t.config.GetString(key.T{Section: s, Option: "type"})
		base := fmt.Sprintf("%s_%s_push", s, drvType)
		action := "push" + drvType
		e := t.newScheduleEntry(action, s, rid.String(), base)
		table = table.Add(e)
	}
	return table
}
