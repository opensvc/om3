package object

import (
	"path/filepath"
	"time"

	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/schedule"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
)

// PrintSchedule display the object scheduling table
func (t *actor) PrintSchedule() schedule.Table {
	return t.Schedules()
}

func (t *actor) lastRunFile(action, rid, base string) string {
	base = "last_" + base
	if rid != "" {
		base = base + "_" + rid
	}
	return filepath.Join(t.VarDir(), "scheduler", base)
}

func (t *actor) lastSuccessFile(action, rid, base string) string {
	return filepath.Join(t.lastRunFile(action, rid, base) + ".success")
}

func (t *actor) loadLast(action, rid, base string) time.Time {
	fpath := t.lastRunFile(action, rid, base)
	return file.ModTime(fpath)
}

func (t *actor) newScheduleEntry(action, keyStr, rid, base string) schedule.Entry {
	k := key.Parse(keyStr)
	def, err := t.config.GetStringStrict(k)
	if err != nil {
		panic(err)
	}
	return schedule.Entry{
		Node:            hostname.Hostname(),
		Path:            t.path,
		Action:          action,
		Last:            t.loadLast(action, rid, base),
		Key:             k.String(),
		Definition:      def,
		LastRunFile:     t.lastRunFile(action, rid, base),
		LastSuccessFile: t.lastSuccessFile(action, rid, base),
	}
}

func (t *actor) Schedules() schedule.Table {
	table := schedule.NewTable(
		t.newScheduleEntry("status", "status_schedule", "", "status"),
		t.newScheduleEntry("compliance_auto", "comp_schedule", "", "comp_check"),
	)
	needResMon := false
	type scheduleOptioner interface {
		ScheduleOptions() resource.ScheduleOptions
	}
	for _, r := range listResources(t) {
		if !needResMon && r.IsMonitored() {
			needResMon = true
		}
		if i, ok := r.(scheduleOptioner); ok {
			opts := i.ScheduleOptions()
			rid := r.RID()
			e := t.newScheduleEntry(opts.Action, key.T{rid, opts.Option}.String(), rid, opts.Base)
			table = table.Add(e)
		}
	}
	if needResMon {
		e := t.newScheduleEntry("resource_monitor", "monitor_schedule", "", "resource_monitor")
		table = table.Add(e)
	}
	if len(listResources(t)) > 0 {
		e := t.newScheduleEntry("push_resinfo", "resinfo_schedule", "", "push_resinfo")
		table = table.Add(e)
	}
	return table
}
