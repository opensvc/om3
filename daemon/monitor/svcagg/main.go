// Package svcagg is responsible for of object.AggregatedStatus
//
// It provides the cluster data ["monitor", "services," <svcname>]
//
// worker ends when context is done or when no more service instance config exist
//
// worker watch on instance status updates to refresh object.AggregatedStatus
package svcagg

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	svcAggStatus struct {
		status object.AggregatedStatus
		path   path.T
		id     string
		nodes  map[string]struct{}

		cmdC         chan *msgbus.Msg
		discoverCmdC chan<- *msgbus.Msg
		dataCmdC     chan<- interface{}

		// instance status map for nodes used to compute AggregatedStatus
		instStatus map[string]instance.Status

		// srcEvent is the source event that create svcAggStatus update
		srcEvent *msgbus.Msg

		ctx context.Context
		log zerolog.Logger
	}
)

// Start launch goroutine svcAggStatus worker for a service
func Start(ctx context.Context, p path.T, cfg instance.Config, svcAggDiscoverCmd chan<- *msgbus.Msg) error {
	id := p.String()
	o := &svcAggStatus{
		status:       object.AggregatedStatus{},
		path:         p,
		id:           id,
		cmdC:         make(chan *msgbus.Msg),
		discoverCmdC: svcAggDiscoverCmd,
		dataCmdC:     daemondata.BusFromContext(ctx),
		instStatus:   make(map[string]instance.Status),
		ctx:          ctx,
		log:          log.Logger.With().Str("func", "svcagg").Stringer("object", p).Logger(),
	}
	go o.worker(cfg.Scope)
	return nil
}

func (o *svcAggStatus) worker(nodes []string) {
	o.log.Debug().Msg("started")
	defer o.log.Debug().Msg("done")
	defer msgbus.DropPendingMsg(o.cmdC, time.Second)
	bus := pubsub.BusFromContext(o.ctx)
	defer msgbus.UnSub(bus, msgbus.SubInstStatus(bus, pubsub.OpUpdate, "svcagg status.update", o.id, o.onEv))
	defer msgbus.UnSub(bus, msgbus.SubCfg(bus, pubsub.OpUpdate, "svcagg cfg.update", o.id, o.onEv))
	defer msgbus.UnSub(bus, msgbus.SubCfg(bus, pubsub.OpDelete, "svcagg cfg.delete", o.id, o.onEv))

	for _, node := range nodes {
		o.instStatus[node] = daemondata.GetInstanceStatus(o.dataCmdC, o.path, node)
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
			o.srcEvent = nil
			switch c := (*ev).(type) {
			case msgbus.CfgUpdated:
				if _, ok := o.instStatus[c.Node]; ok {
					continue
				}
				o.srcEvent = ev
				o.instStatus[c.Node] = daemondata.GetInstanceStatus(o.dataCmdC, o.path, c.Node)
				o.updateStatus()
			case msgbus.CfgDeleted:
				if _, ok := o.instStatus[c.Node]; !ok {
					continue
				}
				delete(o.instStatus, c.Node)
				o.updateStatus()
			case msgbus.InstStatusUpdated:
				if _, ok := o.instStatus[c.Node]; !ok {
					o.log.Debug().Msgf("skip instance change from unknown node: %s", c.Node)
					continue
				}
				o.srcEvent = ev
				o.instStatus[c.Node] = c.Status
				o.updateStatus()
			default:
				o.log.Error().Interface("cmd", *ev).Msg("unexpected cmd")
			}
		}
	}
}

func (o *svcAggStatus) onEv(i interface{}) {
	o.cmdC <- msgbus.NewMsg(i)
}

func (o *svcAggStatus) updateStatus() {
	// TODO update this simple aggregate status compute, perhaps already implemented
	statusCount := make([]uint, 128, 128)
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
	}
	o.update()
}

func (o *svcAggStatus) delete() {
	if err := daemondata.DelServiceAgg(o.dataCmdC, o.path); err != nil {
		o.log.Error().Err(err).Msg("DelServiceAgg")
	}
	o.discoverCmdC <- msgbus.NewMsg(msgbus.MonSvcAggDone{Path: o.path})
}

func (o *svcAggStatus) update() {
	value := o.status.DeepCopy()
	o.log.Debug().Msgf("update avail %s", value.Avail)
	if err := daemondata.SetServiceAgg(o.dataCmdC, o.path, *value, o.srcEvent); err != nil {
		o.log.Error().Err(err).Msg("SetServiceAgg")
	}
}
