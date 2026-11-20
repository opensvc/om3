package object

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

// ValidateConfig validates the configuration, and what its resources say of
// it together, as the schedules of the syncs taking their snapshots at one
// moment.
func (t *actor) ValidateConfig(ctx context.Context) (xconfig.Alerts, error) {
	alerts, err := t.core.ValidateConfig(ctx)
	if err != nil {
		return alerts, err
	}
	syncs := t.ResourcesByDrivergroups([]driver.Group{driver.GroupSync})
	return append(alerts, snapScheduleAlerts(t.config, syncs)...), nil
}

// snapScheduleAlerts warns of the sync resources taking the snapshots they
// copy at one moment, in one run, whose schedule differs from the one of the
// first: the scheduler runs together the syncs due at once only, so their
// snapshots are taken in separate runs. The resources say which they are,
// as resource.SyncSnapshotCopier: a sync.rsync with snap set, a sync.zfs or
// a sync.btrfs, delay_snap not set. A resource with a configuration error
// is reported as such, not here.
func snapScheduleAlerts(cfg *xconfig.T, syncs []resource.Driver) xconfig.Alerts {
	alerts := make(xconfig.Alerts, 0)
	var (
		first         string
		firstSchedule string
	)
	for _, r := range syncs {
		if r.GetConfigurationError() != nil || !resource.CopiesSyncSnapshot(r) {
			continue
		}
		section := r.RID()
		schedule := cfg.GetString(key.New(section, "schedule"))
		if first == "" {
			first, firstSchedule = section, schedule
			continue
		}
		if schedule == firstSchedule {
			continue
		}
		alerts = append(alerts, cfg.NewAlertSchedules(key.New(section, "schedule"), r.DriverID(),
			fmt.Sprintf("differs from %s.schedule: their snapshots are taken in separate runs, not at one moment. Set the same schedule, or delay_snap = true", first)))
	}
	return alerts
}
