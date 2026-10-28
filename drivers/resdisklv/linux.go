//go:build linux

package resdisklv

import (
	"context"
	"fmt"
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

// checkThinPool refuses a logical volume existing already, which the
// provision would take as its own, when the configuration says it is a thin
// volume of a pool it is not one of: a thick volume, or a thin one of
// another pool, would be charged to and watched as the configured pool.
func (t *T) checkThinPool(ctx context.Context) error {
	if t.ThinPool == "" {
		return nil
	}
	info, err := lvm2.NewLV(t.VGName, t.LVName, lvm2.WithLogger(t.Log())).Show(ctx)
	if err != nil {
		return err
	}
	if info.SegType != "thin" {
		return fmt.Errorf("%s/%s exists already, as a %s volume rather than a thin volume of %s: remove it or the thin_pool keyword", t.VGName, t.LVName, info.SegType, t.ThinPool)
	}
	if info.PoolLV != t.ThinPool {
		return fmt.Errorf("%s/%s exists already, as a thin volume of %s rather than of %s: remove it or set the thin_pool keyword to %s", t.VGName, t.LVName, info.PoolLV, t.ThinPool, info.PoolLV)
	}
	return nil
}

// warnThinPoolUsage warns in the status of the resource when the thin pool
// of the volume has most of its data or its metadata used. The pool is the
// one the volume is in, once it exists, which a warning says when it is not
// the configured one: the keyword is read by the provision only.
func (t *T) warnThinPoolUsage(ctx context.Context) {
	pool := t.ThinPool
	if info, err := lvm2.NewLV(t.VGName, t.LVName, lvm2.WithLogger(t.Log())).Show(ctx); err == nil && info.PoolLV != "" {
		if info.PoolLV != t.ThinPool {
			t.StatusLog().Warn("%s/%s is a thin volume of %s, and the thin_pool keyword says %s", t.VGName, t.LVName, info.PoolLV, t.ThinPool)
		}
		pool = info.PoolLV
	}
	info, err := lvm2.NewLV(t.VGName, pool, lvm2.WithLogger(t.Log())).Show(ctx)
	if err != nil {
		t.StatusLog().Warn("thin pool %s/%s: %s", t.VGName, pool, err)
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
			t.StatusLog().Warn("thin pool %s/%s %s is %.0f%% used: a full pool fails the writes of its volumes", t.VGName, pool, usage.name, v)
		}
	}
}
