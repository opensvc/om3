package daemonapi

import (
	"time"

	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/daemon/api"
)

func nullableScheduleTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func scheduleItem(e schedule.Entry) api.ScheduleItem {
	return api.ScheduleItem{
		Kind: "ScheduleItem",
		Meta: api.InstanceMeta{
			Node:   e.Node,
			Object: e.Path.String(),
		},
		Data: api.Schedule{
			Action:             e.Action,
			Key:                e.Key,
			LastRunAt:          nullableScheduleTime(e.LastRunAt),
			MaxParallel:        e.MaxParallel,
			NextRunAt:          nullableScheduleTime(e.NextRunAt),
			Require:            e.Require,
			RequireCollector:   e.RequireCollector,
			RequireProvisioned: e.RequireProvisioned,
			Schedule:           e.Schedule,
		},
	}
}
