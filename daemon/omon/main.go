// Package omon is responsible for of object.Status
//
// It provides the cluster data cluster.objects.<path>
//
// worker ends when context is done or when no more service instance config/status exist
//
// worker watch on instance status, monitor, config updates to refresh object.Status
package omon

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	T struct {
		status object.Status
		path   path.T
		id     string

		discoverCmdC chan<- any
		databus      *daemondata.T

		// instStatus is internal cache for nodes instance status.
		//
		//   The map starts with zero value for the instance.Config node that have
		//   been used to create omon.
		//
		//   Then instStatus map is updated from:
		//      * msgbus.InstanceConfigDeleted
		//      * msgbus.InstanceStatusUpdated,
		//
		//   When the instStatus len is 0, omon returns.
		instStatus map[string]instance.Status

		instMonitor map[string]instance.Monitor

		// srcEvent is the source event that triggered the object status update
		srcEvent any

		ctx context.Context
		log zerolog.Logger

		bus *pubsub.Bus
		sub *pubsub.Subscription

		labelPath pubsub.Label
		labelNode pubsub.Label
		localhost string
	}
)

// Start a goroutine responsible for the status of an object
func Start(ctx context.Context, p path.T, cfg instance.Config, discoverCmdC chan<- any) error {
	id := p.String()
	localhost := hostname.Hostname()
	o := &T{
		status: object.Status{
			Scope:           cfg.Scope,
			FlexTarget:      cfg.FlexTarget,
			FlexMin:         cfg.FlexMin,
			FlexMax:         cfg.FlexMax,
			Orchestrate:     cfg.Orchestrate,
			PlacementPolicy: cfg.PlacementPolicy,
			Priority:        cfg.Priority,
			Topology:        cfg.Topology,
		},
		path:         p,
		id:           id,
		bus:          pubsub.BusFromContext(ctx),
		discoverCmdC: discoverCmdC,
		databus:      daemondata.FromContext(ctx),

		// set initial instStatus value for cfg.Nodename to avoid early termination because of len 0 map
		instStatus: map[string]instance.Status{cfg.Nodename: {}},

		instMonitor: make(map[string]instance.Monitor),
		ctx:         ctx,
		log:         log.Logger.With().Str("func", "omon").Stringer("object", p).Logger(),
		labelNode:   pubsub.Label{"node", localhost},
		labelPath:   pubsub.Label{"path", id},
		localhost:   localhost,
	}
	o.startSubscriptions()
	o.instMonitor = o.databus.GetInstanceMonitorMap(o.path)

	go func() {
		defer func() {
			if err := o.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				o.log.Warn().Err(err).Msg("subscription stop")
			}
		}()
		o.worker()
	}()
	return nil
}

// startSubscriptions starts the subscriptions for omon.
func (o *T) startSubscriptions() {
	sub := o.bus.Sub(o.id + " omon")
	sub.AddFilter(msgbus.InstanceMonitorUpdated{}, o.labelPath)
	sub.AddFilter(msgbus.InstanceConfigUpdated{}, o.labelPath)

	// msgbus.InstanceConfigDeleted is also used to detected msgbus.InstanceStatusDeleted (see forwarded srcEvent to imon)
	sub.AddFilter(msgbus.InstanceConfigDeleted{}, o.labelPath)

	sub.AddFilter(msgbus.InstanceStatusUpdated{}, o.labelPath)
	sub.Start()
	o.sub = sub
}

func (o *T) worker() {
	o.log.Debug().Msg("started")
	defer o.log.Debug().Msg("done")

	// Initiate instStatus values
	for _, i := range o.sub.GetLasts(msgbus.InstanceStatusUpdated{}, o.labelPath, pubsub.Label{"peer", "true"}) {
		switch istatus := i.(type) {
		case msgbus.InstanceStatusUpdated:
			o.instStatus[istatus.Node] = *istatus.Value.DeepCopy()
		}
	}

	o.updateStatus()

	defer o.delete()
	for {
		if len(o.instStatus) == 0 {
			o.log.Info().Msg("no more nodes")
			return
		}
		o.srcEvent = nil
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.sub.C:
			switch c := i.(type) {
			case msgbus.InstanceMonitorUpdated:
				o.srcEvent = i
				o.instMonitor[c.Node] = c.Value
				o.updateStatus()

			case msgbus.InstanceMonitorDeleted:
				o.srcEvent = i
				delete(o.instMonitor, c.Node)
				o.updateStatus()

			case msgbus.InstanceStatusUpdated:
				o.srcEvent = i
				o.instStatus[c.Node] = c.Value
				o.updateStatus()

			case msgbus.InstanceConfigUpdated:
				o.status.Scope = c.Value.Scope
				o.status.FlexTarget = c.Value.FlexTarget
				o.status.FlexMin = c.Value.FlexMin
				o.status.FlexMax = c.Value.FlexMax
				o.status.Orchestrate = c.Value.Orchestrate
				o.status.PlacementPolicy = c.Value.PlacementPolicy
				o.status.Priority = c.Value.Priority
				o.status.Topology = c.Value.Topology
				o.srcEvent = i

				// update local cache for instance status & monitor from cfg node
				// It will be updated on InstanceStatusUpdated, or InstanceMonitorUpdated
				if _, ok := o.instStatus[c.Node]; !ok {
					o.instStatus[c.Node] = instance.Status{}
				}
				if _, ok := o.instMonitor[c.Node]; !ok {
					o.instMonitor[c.Node] = instance.Monitor{}
				}

				o.updateStatus()

			case msgbus.InstanceConfigDeleted:
				if _, ok := o.instStatus[c.Node]; ok {
					delete(o.instStatus, c.Node)
				}
				if _, ok := o.instMonitor[c.Node]; ok {
					delete(o.instMonitor, c.Node)
				}
				o.srcEvent = i
				o.updateStatus()
			}
		}
	}
}

func (o *T) updateStatus() {
	updateAvailOverall := func() {
		statusAvailCount := make([]int, 128, 128)
		statusOverallCount := make([]int, 128, 128)

		agregateStatusFailover := func(states []int) status.T {
			switch states[status.Up] {
			case 0:
				return status.Down
			case 1:
				return status.Up
			default:
				return status.Warn
			}
		}
		agregateStatusFlex := func(states []int) status.T {
			switch {
			case states[status.Up] == 0:
				return status.Down
			case states[status.Up] < o.status.FlexMin:
				return status.Warn
			case states[status.Up] > o.status.FlexMax:
				return status.Warn
			default:
				return status.Up
			}
		}
		agregateStatus := func(states []int) status.T {
			if len(o.instStatus) == 0 {
				return status.NotApplicable
			}
			if len(o.instStatus) == statusAvailCount[status.NotApplicable] {
				return status.NotApplicable
			}
			if states[status.Warn] > 0 {
				return status.Warn
			}
			switch o.status.Topology {
			case topology.Failover:
				return agregateStatusFailover(states)
			case topology.Flex:
				return agregateStatusFlex(states)
			default:
				return status.Undef
			}
		}

		for _, instStatus := range o.instStatus {
			statusAvailCount[instStatus.Avail]++
			statusOverallCount[instStatus.Overall]++
		}

		o.status.UpInstancesCount = statusAvailCount[status.Up]
		o.status.Avail = agregateStatus(statusAvailCount)
		o.status.Overall = agregateStatus(statusOverallCount)
	}

	updateProvisioned := func() {
		o.status.Provisioned = provisioned.Undef
		for _, instStatus := range o.instStatus {
			o.status.Provisioned = o.status.Provisioned.And(instStatus.Provisioned)
		}
	}

	updateFrozen := func() {
		m := map[bool]int{
			true:  0,
			false: 0,
		}
		for _, instStatus := range o.instStatus {
			m[instStatus.Frozen.IsZero()] += 1
		}
		n := len(o.instStatus)
		switch {
		case n == 0:
			o.status.Frozen = "n/a"
		case n == m[false]:
			o.status.Frozen = "frozen"
		case n == m[true]:
			o.status.Frozen = "thawed"
		default:
			o.status.Frozen = "mixed"
		}
	}

	updatePlacementState := func() {
		o.status.PlacementState = placement.NotApplicable
		for node, instMonitor := range o.instMonitor {
			instStatus, ok := o.instStatus[node]
			if !ok {
				o.status.PlacementState = placement.NotApplicable
				break
			}
			if instMonitor.IsLeader && !instStatus.Avail.Is(status.Up, status.NotApplicable) {
				o.status.PlacementState = placement.NonOptimal
				break
			}
			if !instMonitor.IsLeader && !instStatus.Avail.Is(status.Down, status.NotApplicable) {
				o.status.PlacementState = placement.NonOptimal
				break
			}
			o.status.PlacementState = placement.Optimal
		}
	}

	updateAvailOverall()
	updateProvisioned()
	updateFrozen()
	updatePlacementState()
	o.update()
}

func (o *T) delete() {
	o.bus.Pub(msgbus.ObjectStatusDeleted{Path: o.path, Node: o.localhost},
		o.labelPath,
		o.labelNode,
	)
	o.discoverCmdC <- msgbus.ObjectStatusDone{Path: o.path}
}

func (o *T) update() {
	value := o.status.DeepCopy()
	o.log.Debug().Msgf("update avail %s", value.Avail)
	o.bus.Pub(msgbus.ObjectStatusUpdated{Path: o.path, Node: o.localhost, Value: *value, SrcEv: o.srcEvent},
		o.labelPath,
		o.labelNode,
	)
}
