package hbctrl

import (
	"context"
	"time"

	"opensvc.com/opensvc/core/node"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/pubsub"
)

func peerDropWorker(ctx context.Context) {
	databus := daemondata.FromContext(ctx)
	log := daemonlogctx.Logger(ctx).With().Str("Name", "peer-drop").Logger()
	bus := pubsub.BusFromContext(ctx)
	sub := bus.Sub("peer-drop-worker")
	sub.AddFilter(msgbus.CfgFileUpdated{}, pubsub.Label{"path", "cluster"})
	sub.AddFilter(msgbus.CfgFileUpdated{}, pubsub.Label{"path", ""})
	sub.AddFilter(msgbus.HbNodePing{})
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
			log.Error().Err(err).Msgf("drop peer %s", peer)
		}
	}

	delayDropPeer := func(peer string) {
		if databus.GetNodeMonitor(peer).State == node.MonitorStateMaintenance {
			delay := maintenanceGracePeriod
			if drop, ok := dropM[peer]; ok {
				drop.cancel()
				delay = drop.at.Add(maintenanceGracePeriod).Sub(time.Now())
				log.Info().Msgf("maintenance grace period timer reset to %s for %s", delay, peer)
			} else {
				log.Info().Msgf("maintenance grace period timer set to %s for %s", delay, peer)
			}
			dropCtx, cancel := context.WithTimeout(ctx, delay)
			dropM[peer] = dropCall{cancel: cancel, at: time.Now()}
			go func(ctx context.Context, peer string) {
				<-ctx.Done()
				if ctx.Err() == context.Canceled {
					return
				}
				log.Info().Msgf("all hb rx stale for %s and maintenance grace period expired. drop peer data", peer)
				dropPeer(peer)
			}(dropCtx, peer)
		} else {
			log.Info().Msgf("all hb rx stale for %s. drop peer data", peer)
			dropPeer(peer)
		}
	}

	onCfgFileUpdated := func(c msgbus.CfgFileUpdated) {
		if err := config.Reload(); err == nil {
			maintenanceGracePeriod = *config.GetDuration(key.New("node", "maintenance_grace_period"))
			for peer := range dropM {
				delayDropPeer(peer)
			}
		}
	}

	onHbNodePing := func(c msgbus.HbNodePing) {
		peer := c.Node
		if c.Status {
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
			case msgbus.CfgFileUpdated:
				onCfgFileUpdated(c)
			case msgbus.HbNodePing:
				onHbNodePing(c)
			}
		}
	}
}
