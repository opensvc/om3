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

// objectKnownTimeout bounds how long the creation of an object holds its
// answer for the daemon to know it. The daemon learns of an object a moment
// after its configuration is written, the time it takes to read it and start
// watching it, so this only matters when something is wrong.
var objectKnownTimeout = 10 * time.Second

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
