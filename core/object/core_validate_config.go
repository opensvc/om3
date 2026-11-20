package object

import (
	"context"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

// ValidateConfig validates the configuration
func (t *core) ValidateConfig(ctx context.Context) (xconfig.Alerts, error) {
	ctx = actioncontext.WithProps(ctx, actioncontext.ValidateConfig)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return xconfig.Alerts{}, err
	}
	defer unlock()
	alerts, err := t.config.Validate()
	if err != nil {
		return alerts, err
	}
	return append(alerts, snapScheduleAlerts(t.config)...), nil
}

// snapScheduleAlerts warns of the sync resources taking their snapshots at
// one moment, in one run, whose schedule differs from the one of the first:
// the scheduler runs together the syncs due at once only, so their
// snapshots are taken in separate runs. They are the sync.rsync resources
// with snap set and delay_snap not.
func snapScheduleAlerts(cfg *xconfig.T) xconfig.Alerts {
	alerts := make(xconfig.Alerts, 0)
	did := driver.NewID(driver.GroupSync, "rsync")
	var (
		first         string
		firstSchedule string
	)
	for _, section := range cfg.SectionStrings() {
		if !strings.HasPrefix(section, "sync#") {
			continue
		}
		if cfg.GetString(key.New(section, "type")) != "rsync" {
			continue
		}
		if !cfg.GetBool(key.New(section, "snap")) || cfg.GetBool(key.New(section, "delay_snap")) {
			continue
		}
		schedule := cfg.GetString(key.New(section, "schedule"))
		if first == "" {
			first, firstSchedule = section, schedule
			continue
		}
		if schedule == firstSchedule {
			continue
		}
		alerts = append(alerts, cfg.NewAlertSchedules(key.New(section, "schedule"), did,
			fmt.Sprintf("differs from %s.schedule: their snapshots are taken in separate runs, not at one moment. Set the same schedule, or delay_snap = true", first)))
	}
	return alerts
}
