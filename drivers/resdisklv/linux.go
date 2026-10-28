//go:build linux

package resdisklv

import (
	"context"
	"strconv"

	"github.com/opensvc/om3/v3/util/lvm2"
)

// thinPoolUsageWarn is the share of its data or its metadata a thin pool is
// warned about from: a full pool fails the writes of all its volumes, and
// lvm2 grows one only where it is configured to.
const thinPoolUsageWarn = 80.0

func (t *T) lv() LVDriver {
	lv := lvm2.NewLV(
		t.VGName, t.LVName,
		lvm2.WithLogger(t.Log()),
	)
	return lv
}

// warnThinPoolUsage warns in the status of the resource when the thin pool
// of the volume has most of its data or its metadata used.
func (t *T) warnThinPoolUsage(ctx context.Context) {
	info, err := lvm2.NewLV(t.VGName, t.ThinPool, lvm2.WithLogger(t.Log())).Show(ctx)
	if err != nil {
		t.StatusLog().Warn("thin pool %s/%s: %s", t.VGName, t.ThinPool, err)
		return
	}
	for _, usage := range []struct{ name, percent string }{
		{"data", info.DataPercent},
		{"metadata", info.MetadataPercent},
	} {
		v, err := strconv.ParseFloat(usage.percent, 64)
		if err != nil {
			continue
		}
		if v >= thinPoolUsageWarn {
			t.StatusLog().Warn("thin pool %s/%s %s is %.0f%% used: a full pool fails the writes of its volumes", t.VGName, t.ThinPool, usage.name, v)
		}
	}
}
