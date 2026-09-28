package daemonapi

import (
	"sort"
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

func sortScheduleItems(items api.ScheduleItems) {
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Meta.Object != b.Meta.Object {
			return a.Meta.Object < b.Meta.Object
		}
		if a.Meta.Node != b.Meta.Node {
			return a.Meta.Node < b.Meta.Node
		}
		if a.Data.Key != b.Data.Key {
			return a.Data.Key < b.Data.Key
		}
		if a.Data.Action != b.Data.Action {
			return a.Data.Action < b.Data.Action
		}
		return a.Data.Schedule < b.Data.Schedule
	})
}
