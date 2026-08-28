package object

import (
	"github.com/opensvc/om3/v3/core/kwoption"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

func (t *actor) lastRunFile(action, rid, desc string) string {
	base := "last"
	if desc != "" {
		base = base + "_" + desc
	}
	if rid != "" {
		base = base + "_" + rid
	}
	return base
}

func (t *actor) newScheduleEntry(action, keyStr, rid, base string, reqCol, reqProv bool) schedule.Entry {
	k := key.Parse(keyStr)
	def, err := t.config.GetStringStrict(k)
	if err != nil {
		panic(err)
	}
	entry := schedule.Entry{
		Config: schedule.Config{
			Action:             action,
			Key:                k.String(),
			MaxParallel:        1,
			RequireCollector:   reqCol,
			RequireProvisioned: reqProv,
			Schedule:           def,
			StatefileKey:       t.lastRunFile(action, rid, base),
		},
		Node: hostname.Hostname(),
		Path: t.path,
	}
	entry.LastRunAt = entry.LoadLast()
	return entry
}

func (t *actor) Schedules() schedule.Table {
	table := schedule.NewTable(
		t.newScheduleEntry("status", kwoption.ScheduleStatus, "", "status", false, false),
	)
	if t.path.Kind == naming.KindSvc {
		e := t.newScheduleEntry("compliance_auto", kwoption.ScheduleCompliance, "", "comp_check", true, true)
		table = table.Add(e)
	}
	needResMon := false
	type scheduleOptioner interface {
		ScheduleOptions() resource.ScheduleOptions
	}
	for _, r := range listResources(t) {
		if !needResMon && r.IsMonitored() {
			needResMon = true
		}
		if r.IsDisabled() {
			continue
		}
		i, ok := r.(scheduleOptioner)
		if !ok {
			continue
		}
		opts := i.ScheduleOptions()
		if opts.RequireConfirmation {
			continue
		}
		rid := r.RID()
		e := t.newScheduleEntry(opts.Action, key.T{Section: rid, Option: opts.Option}.String(), rid, opts.Base, opts.RequireCollector, opts.RequireProvisioned)
		e.RunDir = opts.RunDir
		e.MaxParallel = opts.MaxParallel
		e.Require = opts.Require
		table = table.Add(e)
	}
	if needResMon {
		e := t.newScheduleEntry("resource_monitor", kwoption.ScheduleMonitor, "", "resource_monitor", false, true)
		table = table.Add(e)
	}
	if len(listResources(t)) > 0 {
		e := t.newScheduleEntry("info", kwoption.ScheduleInfo, "", "info", true, false)
		table = table.Add(e)
	}
	return table
}
