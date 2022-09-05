package object

import (
	"fmt"
	"path/filepath"
	"time"

	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceid"
	"opensvc.com/opensvc/core/schedule"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
)

// PrintSchedule display the object scheduling table
func (t *Node) PrintSchedule() schedule.Table {
	return t.Schedules()
}

func (t *Node) lastFilepath(action string, rid string, base string) string {
	base = "last_" + base
	if rid != "" {
		base = base + "_" + rid
	}
	return filepath.Join(t.VarDir(), "scheduler", base)
}

func (t *Node) lastSuccessFilepath(action string, rid string, base string) string {
	return filepath.Join(t.lastFilepath(action, rid, base) + ".success")
}

func (t *Node) loadLast(action string, rid string, base string) time.Time {
	fpath := t.lastFilepath(action, rid, base)
	return file.ModTime(fpath)
}

func (t *Node) newScheduleEntry(action string, keyStr string, base string) schedule.Entry {
	k := key.Parse(keyStr)
	def, err := t.config.GetStringStrict(k)
	if err != nil {
		panic(err)
	}
	return schedule.Entry{
		Node:       hostname.Hostname(),
		Action:     action,
		Last:       t.loadLast(action, "", base),
		Key:        k.String(),
		Definition: def,
	}
}

func (t *Node) Schedules() schedule.Table {
	table := schedule.NewTable(
		t.newScheduleEntry("pushasset", "asset.schedule", "asset_push"),
		t.newScheduleEntry("reboot", "asset.schedule", "auto_reboot"),
		t.newScheduleEntry("checks", "checks.schedule", "checks_push"),
		t.newScheduleEntry("compliance_auto", "compliance.schedule", "comp_check"),
		t.newScheduleEntry("dequeue_actions", "dequeue_actions.schedule", "dequeue_actions_push"),
		t.newScheduleEntry("pushdisks", "disks.schedule", "disks_push"),
		t.newScheduleEntry("pushpkg", "packages.schedule", "packages_push"),
		t.newScheduleEntry("pushpatch", "patches.schedule", "patches_push"),
		t.newScheduleEntry("rotate_root_pw", "rotate_root_pw.schedule", "rotate_root_pw"),
		t.newScheduleEntry("pushstats", "stats.schedule", "stats_push"),
		t.newScheduleEntry("collect_stats", "stats_collection.schedule", "stats_collection_push"),
		t.newScheduleEntry("sysreport", "sysreport.schedule", "sysreport_push"),
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
		drvType := t.config.GetString(key.T{s, "type"})
		base := fmt.Sprintf("%s_%s_push", s, drvType)
		action := "push" + drvType
		e := t.newScheduleEntry(action, key.T{s, "schedule"}.String(), base)
		table = table.Add(e)
	}
	return table
}
