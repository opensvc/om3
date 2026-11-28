package nmon

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

// volumeFootprint sums up what the status of the local instance of a vol
// object says of the room it takes in its pool: its provisioned state, and
// the size of its filesystem when it has one and is up.
//
// The size is all a status says of a resize, so a volume resized while down,
// or one with no filesystem, changes its pool with no event to tell: the
// load_pool_interval ticker is what sees those.
func volumeFootprint(st instance.Status) string {
	var size int64
	if st.Usage != nil {
		size = st.Usage.Size
	}
	return fmt.Sprintf("%s %d", st.Provisioned, size)
}

// volumeChanged records the footprint of the local instance of a vol object,
// and says if the pools must be loaded again for it.
//
// A footprint seen for the first time is no change when the volume is not
// provisioned: it has taken nothing yet. Provisioned, it is one the pools
// loaded before may not count.
func (t *Manager) volumeChanged(p naming.Path, st instance.Status) bool {
	if t.volumeFootprints == nil {
		t.volumeFootprints = make(map[naming.Path]string)
	}
	footprint := volumeFootprint(st)
	last, seen := t.volumeFootprints[p]
	t.volumeFootprints[p] = footprint
	if !seen {
		return st.Provisioned != provisioned.False && st.Provisioned != provisioned.Undef
	}
	return footprint != last
}

// volumeDeleted forgets the local instance of a vol object, and says if the
// pools must be loaded again for it: an instance never seen is one whose
// volume the pools loaded since the daemon started never counted.
func (t *Manager) volumeDeleted(p naming.Path) bool {
	if _, seen := t.volumeFootprints[p]; !seen {
		return false
	}
	delete(t.volumeFootprints, p)
	return true
}

// onInstanceStatusUpdated loads the pools again when the local instance of
// a vol object changed what it takes in its pool, so the room an unprovision
// freed is published now rather than at the next tick of load_pool_interval,
// a provision meanwhile being refused for a room the pool has.
func (t *Manager) onInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) {
	if c.Node != t.localhost || c.Path.Kind != naming.KindVol {
		return
	}
	if t.volumeChanged(c.Path, c.Value) {
		t.requestLoadPools()
	}
}

// onInstanceStatusDeleted loads the pools again when the local instance of
// a vol object is gone, its volume with it.
func (t *Manager) onInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	if c.Node != t.localhost || c.Path.Kind != naming.KindVol {
		return
	}
	if t.volumeDeleted(c.Path) {
		t.requestLoadPools()
	}
}

// requestLoadPools asks the pool worker to load the pools, without waiting:
// a request already pending is the one that will see what this one is for,
// so a burst of volume changes costs one load, not one each.
func (t *Manager) requestLoadPools() {
	select {
	case t.poolC <- nil:
	default:
	}
}
