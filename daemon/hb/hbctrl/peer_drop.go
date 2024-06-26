package hbctrl

import (
	"context"
	"time"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

// peerDropWorker is responsible for dropping peer data on msgbus.HbNodePing{isAlive: false, Node: <peer>}.
// If <peer> node is in MonitorStateMaintenance state, the drop is delayed until maintenanceGracePeriod is reached.
// The delayed <peer> node drop is canceled on msgbus.HbNodePing{isAlive: true, Node: <peer>}.
func peerDropWorker(ctx context.Context) {
	databus := daemondata.FromContext(ctx)
	log := plog.NewDefaultLogger().Attr("pkg", "daemon/hbctrl:peerDropWorker").WithPrefix("daemon: hbctrl: peer drop: ")
	bus := pubsub.BusFromContext(ctx)
	sub := bus.Sub("daemon.hb.peer_drop_worker")
	sub.AddFilter(&msgbus.ConfigFileUpdated{}, pubsub.Label{"path", "cluster"})
	sub.AddFilter(&msgbus.ConfigFileUpdated{}, pubsub.Label{"path", ""})
	sub.AddFilter(&msgbus.HbNodePing{})
	sub.Start()
	defer sub.Stop()

	type (
		dropCall struct {
			cancel context.CancelFunc
			at     time.Time
		}
	)
	var (
		config *xconfig.T

		maintenanceGracePeriod time.Duration
	)
	if n, err := object.NewNode(object.WithVolatile(true)); err != nil {
		panic(err)
	} else {
		config = n.MergedConfig()
		maintenanceGracePeriod = *config.GetDuration(key.New("node", "maintenance_grace_period"))
	}

	dropM := make(map[string]dropCall)

	dropPeer := func(peer string) {
		err := databus.DropPeerNode(peer)
		if err != nil {
			log.Errorf("drop peer node %s data: %s", peer, err)
		}
	}

	delayDropPeer := func(peer string) {
		if peerMon := node.MonitorData.Get(peer); peerMon != nil && peerMon.State == node.MonitorStateMaintenance {
			delay := maintenanceGracePeriod
			if drop, ok := dropM[peer]; ok {
				drop.cancel()
				delay = drop.at.Add(maintenanceGracePeriod).Sub(time.Now())
				log.Infof("maintenance grace period timer reset to %s for %s", delay, peer)
			} else {
				log.Infof("maintenance grace period timer set to %s for %s", delay, peer)
			}
			dropCtx, cancel := context.WithTimeout(ctx, delay)
			dropM[peer] = dropCall{cancel: cancel, at: time.Now()}
			log.Infof("all hb rx stale for %s in maintenance state => delay drop peer node %s data", peer, peer)
			go func(ctx context.Context, peer string) {
				<-ctx.Done()
				if ctx.Err() == context.Canceled {
					return
				}
				log.Infof("all hb rx stale for %s and maintenance grace period expired => drop peer node %s data", peer, peer)
				dropPeer(peer)
			}(dropCtx, peer)
		} else {
			log.Infof("all hb rx stale for %s => drop peer node %s data", peer, peer)
			dropPeer(peer)
		}
	}

	onConfigFileUpdated := func(c *msgbus.ConfigFileUpdated) {
		if err := config.Reload(); err == nil {
			maintenanceGracePeriod = *config.GetDuration(key.New("node", "maintenance_grace_period"))
			for peer := range dropM {
				delayDropPeer(peer)
			}
		}
	}

	onHbNodePing := func(c *msgbus.HbNodePing) {
		peer := c.Node
		if c.IsAlive {
			if drop, ok := dropM[peer]; ok {
				drop.cancel()
			}
			delete(dropM, peer)
		} else {
			delayDropPeer(peer)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case i := <-sub.C:
			switch c := i.(type) {
			case *msgbus.ConfigFileUpdated:
				onConfigFileUpdated(c)
			case *msgbus.HbNodePing:
				onHbNodePing(c)
			}
		}
	}
}
