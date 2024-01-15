// Package omon is responsible for of object.Status
//
// It provides the cluster data cluster.objects.<path>
//
// worker ends when context is done or when no more service instance config exist
//
// worker is responsible for local imon startup when local instance config is detected
//
// worker watch on instance status, monitor, config updates to refresh object.Status
package omon

import (
	"context"
	"errors"
	"time"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	T struct {
		status object.Status
		path   naming.Path
		id     string

		discoverCmdC chan<- any
		databus      *daemondata.T

		// imonCancel is the cancel function for the local imon we have started
		// We start imon on local instance config received or exists (when instConfig[o.localhost] is created)
		// We cancel imon when local instance config is deleted (when instConfig[o.localhost] is deleted)
		imonCancel  context.CancelFunc
		imonStarter IMonStarter

		// instStatus is internal cache for nodes instance status.
		//
		//   The map starts with zero value for the instance.Config node that have
		//   been used to create omon.
		//
		//   Then instStatus map is updated from:
		//      * msgbus.InstanceConfigDeleted
		//      * msgbus.InstanceStatusUpdated,
		//
		instStatus map[string]instance.Status

		instMonitor map[string]instance.Monitor

		// instConfig track the known instance p configs.
		// It is used to terminate omon (when instConfig len is 0)
		// It is used to start imon (when instConfig[o.localhost] is [re]created)
		instConfig map[string]instance.Config

		// srcEvent is the source event that triggered the object status update
		srcEvent any

		ctx context.Context
		log *plog.Logger

		bus *pubsub.Bus
		sub *pubsub.Subscription

		labelPath pubsub.Label
		labelNode pubsub.Label
		localhost string
	}

	IMonStarter interface {
		Start(parent context.Context, p naming.Path, nodes []string) error
	}
)

// Start a goroutine responsible for the status of an object
func Start(ctx context.Context, p naming.Path, cfg instance.Config, discoverCmdC chan<- any, imonStarter IMonStarter) error {
	id := p.String()
	localhost := hostname.Hostname()
	o := &T{
		status: object.Status{
			Scope:           cfg.Scope,
			FlexTarget:      cfg.FlexTarget,
			FlexMin:         cfg.FlexMin,
			FlexMax:         cfg.FlexMax,
			Orchestrate:     cfg.Orchestrate,
			Pool:            cfg.Pool,
			PlacementPolicy: cfg.PlacementPolicy,
			Priority:        cfg.Priority,
			Size:            cfg.Size,
			Topology:        cfg.Topology,
		},
		path:         p,
		id:           id,
		bus:          pubsub.BusFromContext(ctx),
		discoverCmdC: discoverCmdC,
		databus:      daemondata.FromContext(ctx),

		// set initial instStatus value for cfg.Nodename to avoid early termination because of len 0 map
		instStatus: make(map[string]instance.Status),

		instMonitor: make(map[string]instance.Monitor),

		instConfig: make(map[string]instance.Config),

		ctx: ctx,

		labelNode: pubsub.Label{"node", localhost},
		labelPath: pubsub.Label{"path", id},
		localhost: localhost,

		imonStarter: imonStarter,

		log: naming.LogWithPath(plog.NewDefaultLogger(), p).
			Attr("pkg", "daemon/omon").
			WithPrefix("daemon: omon: " + p.String() + ": "),
	}
	o.startSubscriptions()

	go func() {
		defer func() {
			if err := o.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				o.log.Warnf("subscription stop: %s", err)
			}
		}()
		o.worker()
	}()
	return nil
}

// startSubscriptions starts the subscriptions for omon.
// For each component Updated subscription, we need a component Deleted subscription to maintain internal cache.
func (o *T) startSubscriptions() {
	sub := o.bus.Sub(o.id + " omon")

	sub.AddFilter(&msgbus.InstanceMonitorDeleted{}, o.labelPath)
	sub.AddFilter(&msgbus.InstanceMonitorUpdated{}, o.labelPath)

	// msgbus.InstanceConfigDeleted is also used to detected msgbus.InstanceStatusDeleted (see forwarded srcEvent to imon)
	sub.AddFilter(&msgbus.InstanceConfigDeleted{}, o.labelPath)
	sub.AddFilter(&msgbus.InstanceConfigUpdated{}, o.labelPath)

	sub.AddFilter(&msgbus.InstanceStatusDeleted{}, o.labelPath)
	sub.AddFilter(&msgbus.InstanceStatusUpdated{}, o.labelPath)

	sub.Start()
	o.sub = sub
}

func (o *T) worker() {
	o.log.Infof("started")
	defer o.log.Infof("done")

	// Initiate cache
	for n, v := range instance.MonitorData.GetByPath(o.path) {
		o.instMonitor[n] = *v
	}
	for n, v := range instance.StatusData.GetByPath(o.path) {
		o.instStatus[n] = *v
	}
	for n, v := range instance.ConfigData.GetByPath(o.path) {
		o.instConfig[n] = *v
	}
	if !o.instStatus[o.localhost].UpdatedAt.IsZero() {
		o.srcEvent = &msgbus.InstanceStatusUpdated{Path: o.path, Node: o.localhost, Value: o.instStatus[o.localhost]}
	}

	o.updateStatus()

	if localCfg, ok := o.instConfig[o.localhost]; ok && len(localCfg.Scope) > 0 {
		var err error
		cancel, err := o.startInstanceMonitor(localCfg.Scope)
		if err != nil {
			o.log.Errorf("initial startInstanceMonitor: %s", err)
			cancel()
		} else {
			o.imonCancel = cancel
		}
	}
	defer func() {
		if o.imonCancel != nil {
			o.imonCancel()
			o.imonCancel = nil
		}
		o.delete()
	}()
	for {
		if len(o.instConfig)+len(o.instStatus)+len(o.instMonitor) == 0 {
			o.log.Infof("no more instance config, status and monitor")
			return
		}
		o.srcEvent = nil
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.sub.C:
			switch c := i.(type) {
			case *msgbus.InstanceMonitorUpdated:
				o.srcEvent = c
				o.instMonitor[c.Node] = c.Value
				o.updateStatus()

			case *msgbus.InstanceMonitorDeleted:
				o.srcEvent = c
				delete(o.instMonitor, c.Node)
				o.updateStatus()

			case *msgbus.InstanceStatusDeleted:
				o.srcEvent = c
				delete(o.instStatus, c.Node)
				o.updateStatus()

			case *msgbus.InstanceStatusUpdated:
				o.srcEvent = c
				o.instStatus[c.Node] = c.Value
				o.updateStatus()

			case *msgbus.InstanceConfigUpdated:
				o.status.Scope = c.Value.Scope
				o.status.FlexTarget = c.Value.FlexTarget
				o.status.FlexMin = c.Value.FlexMin
				o.status.FlexMax = c.Value.FlexMax
				o.status.Orchestrate = c.Value.Orchestrate
				o.status.Pool = c.Value.Pool
				o.status.PlacementPolicy = c.Value.PlacementPolicy
				o.status.Priority = c.Value.Priority
				o.status.Size = c.Value.Size
				o.status.Topology = c.Value.Topology
				o.srcEvent = c

				o.instConfig[c.Node] = c.Value

				// update local cache for instance status & monitor from cfg node
				// It will be updated on InstanceStatusUpdated, or InstanceMonitorUpdated
				if c.Node == o.localhost && o.imonCancel == nil && len(c.Value.Scope) > 0 {
					var err error
					cancel, err := o.startInstanceMonitor(c.Value.Scope)
					if err != nil {
						o.log.Errorf("startInstanceMonitor from %+v: %s", c.Value, err)
						cancel()
					} else {
						o.imonCancel = cancel
					}
				}
				o.updateStatus()

			case *msgbus.InstanceConfigDeleted:
				if c.Node == o.localhost && o.imonCancel != nil {
					o.log.Infof("local instance config deleted => cancel associated imon")
					o.imonCancel()
					o.imonCancel = nil
				}
				delete(o.instConfig, c.Node)
				o.srcEvent = c
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
			m[instStatus.FrozenAt.IsZero()]++
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
			if instMonitor.IsHALeader && !instStatus.Avail.Is(status.Up, status.NotApplicable) {
				o.status.PlacementState = placement.NonOptimal
				break
			}
			if !instMonitor.IsHALeader && !instStatus.Avail.Is(status.Down, status.NotApplicable) {
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
	object.StatusData.Unset(o.path)
	o.bus.Pub(&msgbus.ObjectStatusDeleted{Path: o.path, Node: o.localhost},
		o.labelPath,
		o.labelNode,
	)
	o.bus.Pub(&msgbus.ObjectDeleted{Path: o.path, Node: o.localhost},
		o.labelPath,
		o.labelNode,
	)
	o.discoverCmdC <- &msgbus.ObjectStatusDone{Path: o.path}
}

func (o *T) update() {
	o.status.UpdatedAt = time.Now()
	value := o.status.DeepCopy()
	o.log.Debugf("update avail %s", value.Avail)
	object.StatusData.Set(o.path, o.status.DeepCopy())
	o.bus.Pub(&msgbus.ObjectStatusUpdated{Path: o.path, Node: o.localhost, Value: *value, SrcEv: o.srcEvent},
		o.labelPath,
		o.labelNode,
	)
}

func (o *T) startInstanceMonitor(scopes []string) (context.CancelFunc, error) {
	if len(o.status.Scope) == 0 {
		return nil, errors.New("can't call startInstanceMonitor with empty scope")
	}
	o.log.Infof("starting imon worker...")
	ctx, cancel := context.WithCancel(o.ctx)
	if err := o.imonStarter.Start(ctx, o.path, scopes); err != nil {
		o.log.Errorf("unable to start imon worker: %s", err)
		return cancel, err
	}
	return cancel, nil
}
