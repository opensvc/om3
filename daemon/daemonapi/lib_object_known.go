package daemonapi

import (
	"context"
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// objectKnownTimeout bounds how long the creation of an object asked with
// wait_local, and no wait, holds its answer for the daemon to know it. A
// creation asked with a wait holds it as long as the wait instead.
//
// The daemon knows the object once its instance monitor is published, which
// is at the end of the first status of the instance: that status lasts as
// long as its drivers take, many disks, reservations or a slow array
// included, and the monitor is not published before it, for an orchestration
// asked next not to be refused by a peer whose status is not in yet.
var objectKnownTimeout = 30 * time.Second

// objectKnownContext returns the context the creation of an object holds its
// answer in for this daemon to know the object, and what bounds it, for the
// answer past it. Asked to wait, the creation holds as long as the wait: the
// first status of the instance, which the daemon knows the object after, may
// last longer than objectKnownTimeout, which bounds it otherwise.
func objectKnownContext(parent, waitCtx context.Context, waiting bool) (context.Context, context.CancelFunc, string) {
	if waiting {
		return waitCtx, func() {}, "the wait"
	}
	ctx, cancel := context.WithTimeout(parent, objectKnownTimeout)
	return ctx, cancel, objectKnownTimeout.String()
}

// subscribeObjectKnown subscribes to what makes p known to this daemon. It is
// subscribed before the configuration is written, so that nothing happens
// between the write and the wait.
func (a *DaemonAPI) subscribeObjectKnown(name string, p naming.Path) *pubsub.Subscription {
	sub := a.Bus.Sub(name)
	label := pubsub.Label{"path", p.String()}
	sub.AddFilter(&msgbus.ObjectStatusUpdated{}, label)
	sub.AddFilter(&msgbus.InstanceMonitorUpdated{}, label)
	sub.Start()
	return sub
}

// isObjectKnown says whether this daemon knows p the way the requests that
// follow a creation need it to: listed, which is how a selection names it,
// and watched by an instance monitor, which is what an action is asked of.
func isObjectKnown(p naming.Path) bool {
	return object.StatusData.GetByPath(p) != nil && len(instance.MonitorData.GetByPath(p)) > 0
}

// waitObjectKnown holds until this daemon knows p, or ctx ends, and says
// whether it does.
func (a *DaemonAPI) waitObjectKnown(ctx context.Context, sub *pubsub.Subscription, p naming.Path) bool {
	unknown := func() []string {
		if isObjectKnown(p) {
			return nil
		}
		return []string{p.String()}
	}
	return len(a.waitConfigPropagatedTo(ctx, sub, unknown)) == 0
}
