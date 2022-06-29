package object

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"opensvc.com/opensvc/core/schedule"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/timestamp"
)

// PrintSchedule display the object scheduling table
func (t *core) PrintSchedule() schedule.Table {
	return t.Schedules()
}

func (t *core) lastFilepath(action string, rid string, base string) string {
	base = "last_" + base
	if rid != "" {
		base = base + "_" + rid
	}
	return filepath.Join(VarDir(t.path), "scheduler", base)
}

func (t *core) lastSuccessFilepath(action string, rid string, base string) string {
	return filepath.Join(t.lastFilepath(action, rid, base) + ".success")
}

func (t *core) loadLast(action string, rid string, base string) time.Time {
	fpath := t.lastFilepath(action, rid, base)
	b, err := os.ReadFile(fpath)
	if err != nil {
		return time.Unix(0, 0)
	}
	s := strings.TrimSpace(string(b))
	if ti, err := timestamp.Parse(s); err == nil {
		return ti
	}
	loc := time.Now().Location()
	if ti, err := time.ParseInLocation("2006-01-02 15:04:05.9", s, loc); err == nil {
		return ti.UTC()
	}
	return time.Unix(0, 0)
}

func (t *core) newScheduleEntry(action string, keyStr string, base string) schedule.Entry {
	k := key.Parse(keyStr)
	def, err := t.config.GetStringStrict(k)
	if err != nil {
		panic(err)
	}
	return schedule.Entry{
		Node:       hostname.Hostname(),
		Path:       t.path,
		Action:     action,
		Last:       timestamp.New(t.loadLast(action, "", base)),
		Key:        k.String(),
		Definition: def,
	}
}

func (t *core) Schedules() schedule.Table {
	table := schedule.NewTable(
		t.newScheduleEntry("status", "status_schedule", "status"),
		t.newScheduleEntry("compliance_auto", "comp_schedule", "comp_check"),
	)
	needResMon := false
	for _, r := range t.Resources() {
		if !needResMon && r.IsMonitored() {
			needResMon = true
		}
		if i, ok := r.(scheduler); ok {
			table = table.Add(i.Schedules())
		}
	}
	if needResMon {
		e := t.newScheduleEntry("resource_monitor", "monitor_schedule", "resource_monitor")
		table = table.Add(e)
	}
	if len(t.Resources()) > 0 {
		e := t.newScheduleEntry("push_resinfo", "resinfo_schedule", "push_resinfo")
		table = table.Add(e)
	}
	return table
}
