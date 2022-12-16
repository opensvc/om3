// Package smon is responsible for of local instance state
//
//	It provides the cluster data:
//		["cluster", "node", <localhost>, "services", "status", <instance>, "monitor"]
//		["cluster", "node", <localhost>, "services", "smon", <instance>]
//
//	smon are created by the local instcfg, with parent context instcfg context.
//	instcfg done => smon done
//
//	worker watches on local instance status updates to clear reached status
//		=> unsetStatusWhenReached
//		=> orchestrate
//		=> pub new state if change
//
//	worker watches on remote instance smon updates converge global expects
//		=> convergeGlobalExpectFromRemote
//		=> orchestrate
//		=> pub new state if change
package smon

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	smon struct {
		state         instance.Monitor
		previousState instance.Monitor

		path     path.T
		id       string
		ctx      context.Context
		cancel   context.CancelFunc
		cmdC     chan any
		dataCmdC chan<- any
		log      zerolog.Logger

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		// updated data from aggregated status update srcEvent
		instStatus  map[string]instance.Status
		instMonitor map[string]instance.Monitor
		nodeMonitor map[string]cluster.NodeMonitor
		nodeStats   map[string]cluster.NodeStats
		nodeStatus  map[string]cluster.NodeStatus
		scopeNodes  []string

		svcAgg      object.AggregatedStatus
		cancelReady context.CancelFunc
		localhost   string
		change      bool

		sub *pubsub.Subscription
	}

	// cmdOrchestrate can be used from post action go routines
	cmdOrchestrate struct {
		state    string
		newState string
	}
)

var (
	statusDeleted           = "deleted"
	statusDeleting          = "deleting"
	statusFreezeFailed      = "freeze failed"
	statusFreezing          = "freezing"
	statusFrozen            = "frozen"
	statusIdle              = "idle"
	statusProvisioned       = "provisioned"
	statusProvisioning      = "provisioning"
	statusProvisionFailed   = "provision failed"
	statusPurgeFailed       = "purge failed"
	statusReady             = "ready"
	statusStarted           = "started"
	statusStartFailed       = "start failed"
	statusStarting          = "starting"
	statusStopFailed        = "stop failed"
	statusStopped           = "stopped"
	statusStopping          = "stopping"
	statusThawed            = "thawed"
	statusThawedFailed      = "unfreeze failed"
	statusThawing           = "thawing"
	statusUnprovisioned     = "unprovisioned"
	statusUnprovisionFailed = "unprovision failed"
	statusUnprovisioning    = "unprovisioning"
	statusWaitLeader        = "wait leader"
	statusWaitNonLeader     = "wait non-leader"

	localExpectStarted = "started"
	localExpectUnset   = ""

	globalExpectAborted       = "aborted"
	globalExpectFrozen        = "frozen"
	globalExpectPlaced        = "placed"
	globalExpectPlacedAt      = "placed@"
	globalExpectProvisioned   = "provisioned"
	globalExpectPurged        = "purged"
	globalExpectStarted       = "started"
	globalExpectStopped       = "stopped"
	globalExpectThawed        = "thawed"
	globalExpectUnprovisioned = "unprovisioned"
	globalExpectUnset         = ""
)

// Start launch goroutine smon worker for a local instance state
func Start(parent context.Context, p path.T, nodes []string) error {
	ctx, cancel := context.WithCancel(parent)
	id := p.String()

	previousState := instance.Monitor{
		GlobalExpect: globalExpectUnset,
		LocalExpect:  localExpectUnset,
		Status:       statusIdle,
		Restart:      make(map[string]instance.MonitorRestart),
	}
	state := previousState

	o := &smon{
		state:         state,
		previousState: previousState,
		path:          p,
		id:            id,
		ctx:           ctx,
		cancel:        cancel,
		cmdC:          make(chan any),
		dataCmdC:      daemondata.BusFromContext(ctx),
		log:           log.Logger.With().Str("func", "smon").Stringer("object", p).Logger(),
		instStatus:    make(map[string]instance.Status),
		instMonitor:   make(map[string]instance.Monitor),
		nodeStatus:    make(map[string]cluster.NodeStatus),
		nodeStats:     make(map[string]cluster.NodeStats),
		nodeMonitor:   make(map[string]cluster.NodeMonitor),
		localhost:     hostname.Hostname(),
		scopeNodes:    nodes,
		change:        true,
	}

	o.startSubscriptions()

	go func() {
		defer func() {
			msgbus.DropPendingMsg(o.cmdC, time.Second)
			o.sub.Stop()
		}()
		o.worker(nodes)
	}()

	return nil
}

func (o *smon) startSubscriptions() {
	bus := pubsub.BusFromContext(o.ctx)
	sub := bus.Sub(o.id + "smon")
	label := pubsub.Label{"path", o.id}
	sub.AddFilter(msgbus.ObjectAggUpdated{}, label)
	sub.AddFilter(msgbus.SetInstanceMonitor{}, label)
	sub.AddFilter(msgbus.InstanceMonitorUpdated{}, label)
	sub.AddFilter(msgbus.InstanceMonitorDeleted{}, label)
	sub.AddFilter(msgbus.NodeMonitorUpdated{})
	sub.AddFilter(msgbus.NodeStatusUpdated{})
	sub.AddFilter(msgbus.NodeStatsUpdated{})
	sub.Start()
	o.sub = sub
}

// worker watch for local smon updates
func (o *smon) worker(initialNodes []string) {
	defer o.log.Debug().Msg("done")

	for _, node := range initialNodes {
		o.instStatus[node] = daemondata.GetInstanceStatus(o.dataCmdC, o.path, node)
	}
	o.updateIfChange()
	defer o.delete()

	if err := o.crmStatus(); err != nil {
		o.log.Error().Err(err).Msg("error during initial crm status")
	}
	o.log.Debug().Msg("started")
	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.sub.C:
			switch c := i.(type) {
			case msgbus.ObjectAggUpdated:
				o.onObjectAggUpdated(c)
			case msgbus.SetInstanceMonitor:
				o.onSetInstanceMonitorClient(c.Monitor)
			case msgbus.InstanceMonitorUpdated:
				o.onInstanceMonitorUpdated(c)
			case msgbus.InstanceMonitorDeleted:
				o.onInstanceMonitorDeleted(c)
			case msgbus.NodeMonitorUpdated:
				o.onNodeMonitorUpdated(c)
			case msgbus.NodeStatusUpdated:
				o.onNodeStatusUpdated(c)
			case msgbus.NodeStatsUpdated:
				o.onNodeStatsUpdated(c)
			}
		case i := <-o.cmdC:
			switch c := i.(type) {
			case cmdOrchestrate:
				o.needOrchestrate(c)
			}
		}
	}
}

func (o *smon) delete() {
	if err := daemondata.DelInstanceMonitor(o.dataCmdC, o.path); err != nil {
		o.log.Error().Err(err).Msg("DelInstanceMonitor")
	}
}

func (o *smon) update() {
	newValue := o.state
	if err := daemondata.SetInstanceMonitor(o.dataCmdC, o.path, newValue); err != nil {
		o.log.Error().Err(err).Msg("SetInstanceMonitor")
	}
}

func (o *smon) transitionTo(newState string) {
	o.change = true
	o.state.Status = newState
	o.updateIfChange()
}

// updateIfChange log updates and publish new state value when changed
func (o *smon) updateIfChange() {
	if !o.change {
		return
	}
	o.change = false
	now := time.Now()
	previousVal := o.previousState
	newVal := o.state
	fromGeS, toGeS := o.logFromTo(previousVal.GlobalExpect, newVal.GlobalExpect)
	if newVal.GlobalExpect != previousVal.GlobalExpect {
		// Don't update GlobalExpectUpdated here
		// GlobalExpectUpdated is updated only during cmdSetInstanceMonitorClient and
		// its value is used for convergeGlobalExpectFromRemote
		o.loggerWithState().Info().Msgf("change monitor global expect %s -> %s", fromGeS, toGeS)
	}
	if newVal.LocalExpect != previousVal.LocalExpect {
		o.state.LocalExpectUpdated = now
		from, to := o.logFromTo(previousVal.LocalExpect, newVal.LocalExpect)
		o.loggerWithState().Info().Msgf("change monitor local expect %s -> %s", from, to)
	}
	if newVal.Status != previousVal.Status {
		o.state.StatusUpdated = now
		o.loggerWithState().Info().Msgf("change monitor state %s -> %s", previousVal.Status, newVal.Status)
	}
	if newVal.IsLeader != previousVal.IsLeader {
		o.loggerWithState().Info().Msgf("change leader state %t -> %t", previousVal.IsLeader, newVal.IsLeader)
	}
	if newVal.IsHALeader != previousVal.IsHALeader {
		o.loggerWithState().Info().Msgf("change ha leader state %t -> %t", previousVal.IsHALeader, newVal.IsHALeader)
	}
	o.previousState = o.state
	o.update()
}

func (o *smon) hasOtherNodeActing() bool {
	for remoteNode, remoteInstMonitor := range o.instMonitor {
		if remoteNode == o.localhost {
			continue
		}
		if strings.HasSuffix(remoteInstMonitor.Status, "ing") {
			return true
		}
	}
	return false
}

func (o *smon) createPendingWithCancel() {
	o.pendingCtx, o.pendingCancel = context.WithCancel(o.ctx)
}

func (o *smon) createPendingWithDuration(duration time.Duration) {
	o.pendingCtx, o.pendingCancel = context.WithTimeout(o.ctx, duration)
}

func (o *smon) clearPending() {
	if o.pendingCancel != nil {
		o.pendingCancel()
		o.pendingCancel = nil
		o.pendingCtx = nil
	}
}

func (o *smon) logFromTo(from, to string) (string, string) {
	if from == "" {
		from = "unset"
	}
	if to == "" {
		to = "unset"
	}
	return from, to
}

func (o *smon) loggerWithState() *zerolog.Logger {
	ctx := o.log.With()
	if o.state.GlobalExpect != globalExpectUnset {
		ctx.Str("global_expect", o.state.GlobalExpect)
	}
	if o.state.LocalExpect != statusIdle && o.state.LocalExpect != localExpectUnset {
		ctx.Str("local_expect", o.state.LocalExpect)
	}
	stateLogger := ctx.Logger()
	return &stateLogger
}
