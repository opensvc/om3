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
	Manager struct {
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
func Start(ctx context.Context, subQS pubsub.QueueSizer, p naming.Path, cfg instance.Config, discoverCmdC chan<- any, imonStarter IMonStarter) error {
	id := p.String()
	localhost := hostname.Hostname()
	t := &Manager{
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
	t.startSubscriptions(subQS)

	go func() {
		defer func() {
			if err := t.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				t.log.Warnf("subscription stop: %s", err)
			}
		}()
		t.worker()
	}()
	return nil
}

// startSubscriptions starts the subscriptions for omon.
// For each component Updated subscription, we need a component Deleted subscription to maintain internal cache.
func (t *Manager) startSubscriptions(subQS pubsub.QueueSizer) {
	sub := t.bus.Sub("daemon.omon "+t.id, subQS)

	sub.AddFilter(&msgbus.InstanceMonitorDeleted{}, t.labelPath)
	sub.AddFilter(&msgbus.InstanceMonitorUpdated{}, t.labelPath)

	// msgbus.InstanceConfigDeleted is also used to detected msgbus.InstanceStatusDeleted (see forwarded srcEvent to imon)
	sub.AddFilter(&msgbus.InstanceConfigDeleted{}, t.labelPath)
	sub.AddFilter(&msgbus.InstanceConfigUpdated{}, t.labelPath)

	sub.AddFilter(&msgbus.InstanceStatusDeleted{}, t.labelPath)
	sub.AddFilter(&msgbus.InstanceStatusUpdated{}, t.labelPath)

	sub.Start()
	t.sub = sub
}

func (t *Manager) worker() {
	t.log.Infof("started")
	defer t.log.Infof("done")

	// Initiate cache
	for n, v := range instance.MonitorData.GetByPath(t.path) {
		t.instMonitor[n] = *v
	}
	for n, v := range instance.StatusData.GetByPath(t.path) {
		t.instStatus[n] = *v
	}
	for n, v := range instance.ConfigData.GetByPath(t.path) {
		t.instConfig[n] = *v
	}
	if !t.instStatus[t.localhost].UpdatedAt.IsZero() {
		t.srcEvent = &msgbus.InstanceStatusUpdated{Path: t.path, Node: t.localhost, Value: t.instStatus[t.localhost]}
	}

	t.updateStatus()

	if localCfg, ok := t.instConfig[t.localhost]; ok && len(localCfg.Scope) > 0 {
		var err error
		cancel, err := t.startInstanceMonitor(localCfg.Scope)
		if err != nil {
			t.log.Errorf("initial startInstanceMonitor: %s", err)
			cancel()
		} else {
			t.imonCancel = cancel
		}
	}
	defer func() {
		if t.imonCancel != nil {
			t.imonCancel()
			t.imonCancel = nil
		}
		t.delete()
	}()
	for {
		if len(t.instConfig)+len(t.instStatus)+len(t.instMonitor) == 0 {
			t.log.Infof("no more instance config, status and monitor")
			return
		}
		t.srcEvent = nil
		select {
		case <-t.ctx.Done():
			return
		case i := <-t.sub.C:
			switch c := i.(type) {
			case *msgbus.InstanceMonitorUpdated:
				t.srcEvent = c
				t.instMonitor[c.Node] = c.Value
				t.updateStatus()

			case *msgbus.InstanceMonitorDeleted:
				t.srcEvent = c
				delete(t.instMonitor, c.Node)
				t.updateStatus()

			case *msgbus.InstanceStatusDeleted:
				t.srcEvent = c
				delete(t.instStatus, c.Node)
				t.updateStatus()

			case *msgbus.InstanceStatusUpdated:
				t.srcEvent = c
				t.instStatus[c.Node] = c.Value
				t.updateStatus()

			case *msgbus.InstanceConfigUpdated:
				t.status.Scope = c.Value.Scope
				t.status.FlexTarget = c.Value.FlexTarget
				t.status.FlexMin = c.Value.FlexMin
				t.status.FlexMax = c.Value.FlexMax
				t.status.Orchestrate = c.Value.Orchestrate
				t.status.Pool = c.Value.Pool
				t.status.PlacementPolicy = c.Value.PlacementPolicy
				t.status.Priority = c.Value.Priority
				t.status.Size = c.Value.Size
				t.status.Topology = c.Value.Topology
				t.srcEvent = c

				t.instConfig[c.Node] = c.Value

				// update local cache for instance status & monitor from cfg node
				// It will be updated on InstanceStatusUpdated, or InstanceMonitorUpdated
				if c.Node == t.localhost && t.imonCancel == nil && len(c.Value.Scope) > 0 {
					var err error
					cancel, err := t.startInstanceMonitor(c.Value.Scope)
					if err != nil {
						t.log.Errorf("startInstanceMonitor from %+v: %s", c.Value, err)
						cancel()
					} else {
						t.imonCancel = cancel
					}
				}
				t.updateStatus()

			case *msgbus.InstanceConfigDeleted:
				if c.Node == t.localhost && t.imonCancel != nil {
					t.log.Infof("local instance config deleted => cancel associated imon")
					t.imonCancel()
					t.imonCancel = nil
				}
				delete(t.instConfig, c.Node)
				t.srcEvent = c
				t.updateStatus()
			}
		}
	}
}

func (t *Manager) updateStatus() {
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
			case states[status.Up] < t.status.FlexMin:
				return status.Warn
			case states[status.Up] > t.status.FlexMax:
				return status.Warn
			default:
				return status.Up
			}
		}
		agregateStatus := func(states []int) status.T {
			if len(t.instStatus) == 0 {
				return status.NotApplicable
			}
			if len(t.instStatus) == statusAvailCount[status.NotApplicable] {
				return status.NotApplicable
			}
			if states[status.Warn] > 0 {
				return status.Warn
			}
			switch t.status.Topology {
			case topology.Failover:
				return agregateStatusFailover(states)
			case topology.Flex:
				return agregateStatusFlex(states)
			default:
				return status.Undef
			}
		}

		for _, instStatus := range t.instStatus {
			statusAvailCount[instStatus.Avail]++
			statusOverallCount[instStatus.Overall]++
		}

		t.status.UpInstancesCount = statusAvailCount[status.Up]
		t.status.Avail = agregateStatus(statusAvailCount)
		t.status.Overall = agregateStatus(statusOverallCount)
	}

	updateProvisioned := func() {
		t.status.Provisioned = provisioned.Undef
		for _, instStatus := range t.instStatus {
			t.status.Provisioned = t.status.Provisioned.And(instStatus.Provisioned)
		}
	}

	updateFrozen := func() {
		m := map[bool]int{
			true:  0,
			false: 0,
		}
		for _, instStatus := range t.instStatus {
			m[instStatus.FrozenAt.IsZero()]++
		}
		n := len(t.instStatus)
		switch {
		case n == 0:
			t.status.Frozen = "n/a"
		case n == m[false]:
			t.status.Frozen = "frozen"
		case n == m[true]:
			t.status.Frozen = "thawed"
		default:
			t.status.Frozen = "mixed"
		}
	}

	updatePlacementState := func() {
		t.status.PlacementState = placement.NotApplicable
		for node, instMonitor := range t.instMonitor {
			instStatus, ok := t.instStatus[node]
			if !ok {
				t.status.PlacementState = placement.NotApplicable
				break
			}
			if instMonitor.IsHALeader && !instStatus.Avail.Is(status.Up, status.NotApplicable) {
				t.status.PlacementState = placement.NonOptimal
				break
			}
			if !instMonitor.IsHALeader && !instStatus.Avail.Is(status.Down, status.NotApplicable) {
				t.status.PlacementState = placement.NonOptimal
				break
			}
			t.status.PlacementState = placement.Optimal
		}
	}

	updateAvailOverall()
	updateProvisioned()
	updateFrozen()
	updatePlacementState()
	t.update()
}

func (t *Manager) delete() {
	object.StatusData.Unset(t.path)
	t.bus.Pub(&msgbus.ObjectStatusDeleted{Path: t.path, Node: t.localhost},
		t.labelPath,
		t.labelNode,
	)
	t.bus.Pub(&msgbus.ObjectDeleted{Path: t.path, Node: t.localhost},
		t.labelPath,
		t.labelNode,
	)
	t.discoverCmdC <- &msgbus.ObjectStatusDone{Path: t.path}
}

func (t *Manager) update() {
	t.status.UpdatedAt = time.Now()
	value := t.status.DeepCopy()
	t.log.Debugf("update avail %s", value.Avail)
	object.StatusData.Set(t.path, t.status.DeepCopy())
	t.bus.Pub(&msgbus.ObjectStatusUpdated{Path: t.path, Node: t.localhost, Value: *value, SrcEv: t.srcEvent},
		t.labelPath,
		t.labelNode,
	)
}

func (t *Manager) startInstanceMonitor(scopes []string) (context.CancelFunc, error) {
	if len(t.status.Scope) == 0 {
		return nil, errors.New("can't call startInstanceMonitor with empty scope")
	}
	t.log.Infof("starting imon worker...")
	ctx, cancel := context.WithCancel(t.ctx)
	if err := t.imonStarter.Start(ctx, t.path, scopes); err != nil {
		t.log.Errorf("unable to start imon worker: %s", err)
		return cancel, err
	}
	return cancel, nil
}
