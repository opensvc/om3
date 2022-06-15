// Package svcAggStatus is responsible for of object.AggregatedStatus
//
// It provides the cluster data ["monitor", "services," <svcname>]
//
// worker ends when context is done or when no more service instance config exist
//
// worker watch on instance status updates to refresh object.AggregatedStatus
//
package svcagg

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	ps "opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	svcAggStatus struct {
		status object.AggregatedStatus
		path   path.T
		id     string
		nodes  map[string]struct{}

		cmdC         chan *moncmd.T
		discoverCmdC chan<- *moncmd.T
		dataCmdC     chan<- interface{}

		// instance status map for nodes used to compute AggregatedStatus
		instStatus map[string]instance.Status

		ctx context.Context
		log zerolog.Logger
	}
)

// Start launch goroutine svcAggStatus worker for a service
func Start(ctx context.Context, p path.T, cfg instance.Config, svcAggDiscoverCmd chan<- *moncmd.T) error {
	id := p.String()
	o := &svcAggStatus{
		status:       object.AggregatedStatus{},
		path:         p,
		id:           id,
		cmdC:         make(chan *moncmd.T),
		discoverCmdC: svcAggDiscoverCmd,
		dataCmdC:     daemonctx.DaemonDataCmd(ctx),
		instStatus:   make(map[string]instance.Status),
		ctx:          ctx,
		log:          daemonctx.Logger(ctx).With().Str("_svcagg", id).Logger(),
	}
	go o.worker(cfg.Scope)
	return nil
}

func (o *svcAggStatus) worker(nodes []string) {
	o.log.Info().Msg("started")
	defer o.log.Info().Msg("done")
	defer moncmd.DropPendingCmd(o.cmdC, time.Second)
	c := daemonctx.DaemonPubSubCmd(o.ctx)
	defer ps.UnSub(c, ps.SubInstStatus(c, pubsub.OpUpdate, "svcagg status.update", o.id, o.onEv))
	defer ps.UnSub(c, ps.SubCfg(c, pubsub.OpUpdate, "svcagg cfg.update", o.id, o.onEv))
	defer ps.UnSub(c, ps.SubCfg(c, pubsub.OpDelete, "svcagg cfg.delete", o.id, o.onEv))

	for _, node := range nodes {
		o.instStatus[node] = daemondata.GelInstanceStatus(o.dataCmdC, o.path, node)
	}
	o.update()
	defer o.delete()
	for {
		if len(o.instStatus) == 0 {
			o.log.Info().Msg("no more nodes")
			return
		}
		select {
		case <-o.ctx.Done():
			return
		case ev := <-o.cmdC:
			switch c := (*ev).(type) {
			case moncmd.CfgUpdated:
				if _, ok := o.instStatus[c.Node]; ok {
					continue
				}
				o.instStatus[c.Node] = daemondata.GelInstanceStatus(o.dataCmdC, o.path, c.Node)
				o.updateStatus()
			case moncmd.CfgDeleted:
				if _, ok := o.instStatus[c.Node]; !ok {
					continue
				}
				delete(o.instStatus, c.Node)
				o.updateStatus()
			case moncmd.InstStatusUpdated:
				if _, ok := o.instStatus[c.Node]; !ok {
					o.log.Info().Msgf("skipped instance change on unknown node: %s", c.Node)
					continue
				}
				o.instStatus[c.Node] = c.Status
				o.updateStatus()
			default:
				o.log.Error().Interface("cmd", *ev).Msg("unexpected cmd")
			}
		}
	}
}

func (o *svcAggStatus) onEv(i interface{}) {
	o.cmdC <- moncmd.New(i)
}

func (o *svcAggStatus) updateStatus() {
	// TODO update this simple aggregate status compute, perhaps already implemented
	statusCount := make([]uint, 10, 10)
	var newAvail status.T
	for _, instStatus := range o.instStatus {
		statusCount[instStatus.Avail]++
	}
	if statusCount[status.Warn] > 0 {
		newAvail = status.Warn
	} else if statusCount[status.Up] > 0 {
		newAvail = status.Up
	} else if statusCount[status.Down] > 0 {
		newAvail = status.Down
	} else {
		newAvail = status.Undef
	}
	if o.status.Avail != newAvail {
		o.status.Avail = newAvail
		o.log.Info().Msgf("updated status avail to %s", o.status.Avail)
		o.update()
	}
}

func (o *svcAggStatus) delete() {
	if err := daemondata.DelServiceAgg(o.dataCmdC, o.path); err != nil {
		o.log.Error().Err(err).Msg("DelServiceAgg")
	}
	o.discoverCmdC <- moncmd.New(moncmd.MonSvcAggDone{Path: o.path})
}

func (o *svcAggStatus) update() {
	value := o.status.DeepCopy()
	o.log.Info().Msgf("update avail %s", value.Avail)
	if err := daemondata.SetServiceAgg(o.dataCmdC, o.path, *value); err != nil {
		o.log.Error().Err(err).Msg("SetServiceAgg")
	}
}
