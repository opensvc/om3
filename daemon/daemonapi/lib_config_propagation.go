package daemonapi

import (
	"context"
	"sort"
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// configPropagationSafetyTick is how often the propagation of a
// configuration is looked at when no event says it changed. A configuration
// reaches a peer in a quarter of a second, and the events of its instance
// configurations say so as it happens: this only covers what no event says,
// as a node dropping out of the cluster while it is waited for.
var configPropagationSafetyTick = time.Second

// subscribeConfigPropagation subscribes to what changes where the
// configuration of p has landed. It is subscribed before the configuration is
// written, so that no landing happens between the write and the wait.
func (a *DaemonAPI) subscribeConfigPropagation(name string, p naming.Path) *pubsub.Subscription {
	sub := a.Bus.Sub(name)
	label := pubsub.Label{"path", p.String()}
	sub.AddFilter(&msgbus.InstanceConfigUpdated{}, label)
	sub.AddFilter(&msgbus.InstanceConfigDeleted{}, label)
	sub.AddFilter(&msgbus.NodeMonitorDeleted{})
	sub.Start()
	return sub
}

// waitConfigPropagated holds until the configuration of p that carries the
// timestamp at has reached every live node of the object, or ctx ends, and
// returns the nodes it has not reached.
//
// It looks again on every event of an instance configuration of the object,
// so it answers as the last node lands rather than at the next poll: at a
// quarter of a second a landing takes, a poll would be most of the wait.
func (a *DaemonAPI) waitConfigPropagated(ctx context.Context, sub *pubsub.Subscription, p naming.Path, at time.Time) []string {
	ticker := time.NewTicker(configPropagationSafetyTick)
	defer ticker.Stop()
	for {
		lagging := a.configLaggards(p, at)
		if len(lagging) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return lagging
		case <-sub.C:
		case <-ticker.C:
		}
	}
}

// configLaggards returns the live nodes of the scope of p still holding a
// configuration older than at.
//
// A peer fetches a configuration and keeps the timestamp it was written with,
// so the one it holds is the one written at or after that time.
//
// The scope is the one of the configuration written, as this node read it
// back: it may name nodes the configuration it replaces did not, which have
// to receive it too, and until this node has read it back, the scope is not
// known and this node is the one it has not reached.
//
// A node is live while this daemon holds its data, which it drops when all
// the heartbeats of the node went stale: a node that is down fetches the
// configuration when it comes back, and is not waited for.
func (a *DaemonAPI) configLaggards(p naming.Path, at time.Time) []string {
	configs := instance.ConfigData.GetByPath(p)
	local, ok := configs[a.localhost]
	if !ok || local.UpdatedAt.Before(at) {
		return []string{a.localhost}
	}
	l := make([]string, 0)
	for _, nodename := range local.Scope {
		if nodename == a.localhost {
			continue
		}
		if node.MonitorData.GetByNode(nodename) == nil {
			continue
		}
		if cfg, ok := configs[nodename]; !ok || cfg.UpdatedAt.Before(at) {
			l = append(l, nodename)
		}
	}
	sort.Strings(l)
	return l
}
