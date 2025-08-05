package object

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/core/schedule"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
)

func (t *Node) lastRunFile(action, rid, base string) string {
	base = "last_" + base
	if rid != "" {
		base = base + "_" + rid
	}
	return filepath.Join(t.VarDir(), "scheduler", base)
}

func (t *Node) lastSuccessFile(action, rid, base string) string {
	return filepath.Join(t.lastRunFile(action, rid, base) + ".success")
}

func (t *Node) loadLast(action, rid, base string) time.Time {
	fpath := t.lastRunFile(action, rid, base)
	return file.ModTime(fpath)
}

func (t *Node) newScheduleEntry(action, section, rid, base string) schedule.Entry {
	k := key.T{Section: section, Option: "schedule"}
	def, err := t.config.GetStringStrict(k)
	if err != nil {
		panic(err)
	}
	return schedule.Entry{
		Node:             hostname.Hostname(),
		Action:           action,
		LastRunAt:        t.loadLast(action, rid, base),
		Key:              k.String(),
		Schedule:         def,
		LastRunFile:      t.lastRunFile(action, rid, base),
		LastSuccessFile:  t.lastSuccessFile(action, rid, base),
		RequireCollector: true,
	}
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
		t.newScheduleEntry("dequeue_actions", "dequeue_actions", "", "dequeue_actions_push"),
	)
	type scheduleOptioner interface {
		ScheduleOptions() resource.ScheduleOptions
	}
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
