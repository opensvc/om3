package resipnetns

import (
	"context"
	"fmt"
	"time"

	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resip"
	"github.com/opensvc/om3/v3/util/ping"
)

// ProvisionConflicts says the address answers already, from another host or
// another object of this one: the provision would bring it up a second time.
// An address still to draw from a network is not checked: it is drawn free.
//
// A provision that does not start the address, off the leader of the
// placement and not standby, brings nothing up a second time: the address
// answering is then the one the leader of this object started, and the start
// of this node checks it again. An address up on the device of the resource
// is its own, as the abort of a start counts it.
func (t *T) ProvisionConflicts(ctx context.Context) []string {
	ip := t.ipaddr()
	if ip == nil {
		return nil
	}
	if !resip.ProvisionStarts(ctx, t.IsStandby()) {
		return nil
	}
	if t.statusWithIPAddrCacheTrust(ctx) == status.Up {
		return nil
	}
	if alive, err := ping.Ping(ip.String(), 5*time.Second); err == nil && alive {
		return []string{fmt.Sprintf("%s answers", ip)}
	}
	return nil
}
