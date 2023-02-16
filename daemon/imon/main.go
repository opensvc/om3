// Package imon is responsible for of local instance state
//
//	It provides the cluster data:
//		["cluster", "node", <localhost>, "services", "status", <instance>, "monitor"]
//		["cluster", "node", <localhost>, "services", "imon", <instance>]
//
//	imon are created by the local instcfg, with parent context instcfg context.
//	instcfg done => imon done
//
//	worker watches on local instance status updates to clear reached status
//		=> unsetStatusWhenReached
//		=> orchestrate
//		=> pub new state if change
//
//	worker watches on remote instance imon updates converge global expects
//		=> convergeGlobalExpectFromRemote
//		=> orchestrate
//		=> pub new state if change
package imon

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	imon struct {
		state         instance.Monitor
		previousState instance.Monitor

		path    path.T
		id      string
		ctx     context.Context
		cancel  context.CancelFunc
		cmdC    chan any
		databus *daemondata.T
		log     zerolog.Logger

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		// updated data from object status update srcEvent
		instConfig    instance.Config
		instStatus    map[string]instance.Status
		instMonitor   map[string]instance.Monitor
		nodeMonitor   map[string]node.Monitor
		nodeStats     map[string]node.Stats
		nodeStatus    map[string]node.Status
		scopeNodes    []string
		readyDuration time.Duration

		objStatus   object.Status
		cancelReady context.CancelFunc
		localhost   string
		change      bool

		sub *pubsub.Subscription

		pubsubBus *pubsub.Bus

		// waitConvergedOrchestrationMsg is a map indexed by nodename to latest waitConvergedOrchestrationMsg.
		// It is used while we are waiting for orchestration reached
		waitConvergedOrchestrationMsg map[string]string

		acceptedOrchestrationId string
	}

	// cmdOrchestrate can be used from post action go routines
	cmdOrchestrate struct {
		state    instance.MonitorState
		newState instance.MonitorState
	}

	imonFactory struct{}
)

// Start creates a new imon and starts worker goroutine to manage local instance monitor
func (i imonFactory) Start(parent context.Context, p path.T, nodes []string) error {
	return start(parent, p, nodes)
}

var (
	Factory imonFactory

	defaultReadyDuration = 5 * time.Second
)

// start launch goroutine imon worker for a local instance state
func start(parent context.Context, p path.T, nodes []string) error {
	ctx, cancel := context.WithCancel(parent)
	id := p.String()

	previousState := instance.Monitor{
		LocalExpect:  instance.MonitorLocalExpectNone,
		GlobalExpect: instance.MonitorGlobalExpectNone,
		State:        instance.MonitorStateIdle,
		Resources:    make(map[string]instance.ResourceMonitor),
		StateUpdated: time.Now(),
	}
	state := previousState
	databus := daemondata.FromContext(ctx)

	o := &imon{
		state:         state,
		previousState: previousState,
		path:          p,
		id:            id,
		ctx:           ctx,
		cancel:        cancel,
		cmdC:          make(chan any),
		databus:       databus,
		pubsubBus:     pubsub.BusFromContext(ctx),
		log:           log.Logger.With().Str("func", "imon").Stringer("object", p).Logger(),
		instStatus:    make(map[string]instance.Status),
		instMonitor:   make(map[string]instance.Monitor),
		localhost:     hostname.Hostname(),
		scopeNodes:    nodes,
		change:        true,
		readyDuration: defaultReadyDuration,

		waitConvergedOrchestrationMsg: make(map[string]string),
	}

	o.startSubscriptions()
	o.nodeStatus = databus.GetNodeStatusMap()
	o.nodeStats = databus.GetNodeStatsMap()
	o.nodeMonitor = databus.GetNodeMonitorMap()
	o.instMonitor = databus.GetInstanceMonitorMap(o.path)
	o.instConfig = databus.GetInstanceConfig(o.path, o.localhost)
	o.initResourceMonitor()

	go func() {
		defer func() {
			msgbus.DropPendingMsg(o.cmdC, time.Second)
			err := o.sub.Stop()
			if err != nil {
				o.log.Error().Err(err).Msg("sub.stop")
			}
		}()
		o.worker(nodes)
	}()

	return nil
}

func (o *imon) startSubscriptions() {
	sub := o.pubsubBus.Sub(o.id + " imon")
	label := pubsub.Label{"path", o.id}
	nodeLabel := pubsub.Label{"node", o.localhost}
	sub.AddFilter(msgbus.ObjectStatusUpdated{}, label)
	sub.AddFilter(msgbus.ProgressInstanceMonitor{}, label)
	sub.AddFilter(msgbus.SetInstanceMonitor{}, label)
	sub.AddFilter(msgbus.NodeConfigUpdated{}, nodeLabel)
	sub.AddFilter(msgbus.NodeMonitorUpdated{})
	sub.AddFilter(msgbus.NodeStatusUpdated{})
	sub.AddFilter(msgbus.NodeStatsUpdated{})
	sub.Start()
	o.sub = sub
}

// worker watch for local imon updates
func (o *imon) worker(initialNodes []string) {
	defer o.log.Debug().Msg("done")

	for _, initialNode := range initialNodes {
		o.instStatus[initialNode] = o.databus.GetInstanceStatus(o.path, initialNode)
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
			case msgbus.ObjectStatusUpdated:
				o.onObjectStatusUpdated(c)
			case msgbus.ProgressInstanceMonitor:
				o.onProgressInstanceMonitor(c)
			case msgbus.SetInstanceMonitor:
				o.onSetInstanceMonitor(c)
			case msgbus.NodeConfigUpdated:
				o.onNodeConfigUpdated(c)
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

func (o *imon) delete() {
	if err := o.databus.DelInstanceMonitor(o.path); err != nil {
		o.log.Error().Err(err).Msg("DelInstanceMonitor")
	}
}

func (o *imon) update() {
	newValue := o.state
	if err := o.databus.SetInstanceMonitor(o.path, newValue); err != nil {
		o.log.Error().Err(err).Msg("SetInstanceMonitor")
	}
}

func (o *imon) transitionTo(newState instance.MonitorState) {
	o.change = true
	o.state.State = newState
	o.updateIfChange()
}

// updateIfChange log updates and publish new state value when changed
func (o *imon) updateIfChange() {
	if !o.change {
		return
	}
	o.change = false
	now := time.Now()
	previousVal := o.previousState
	newVal := o.state
	if newVal.GlobalExpect != previousVal.GlobalExpect {
		// Don't update GlobalExpectUpdated here
		// GlobalExpectUpdated is updated only during cmdSetInstanceMonitorClient and
		// its value is used for convergeGlobalExpectFromRemote
		o.loggerWithState().Info().Msgf("change monitor global expect %s -> %s", previousVal.GlobalExpect, newVal.GlobalExpect)
	}
	if newVal.LocalExpect != previousVal.LocalExpect {
		o.state.LocalExpectUpdated = now
		o.loggerWithState().Info().Msgf("change monitor local expect %s -> %s", previousVal.LocalExpect, newVal.LocalExpect)
	}
	if newVal.State != previousVal.State {
		o.state.StateUpdated = now
		o.loggerWithState().Info().Msgf("change monitor state %s -> %s", previousVal.State, newVal.State)
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

func (o *imon) hasOtherNodeActing() bool {
	for remoteNode, remoteInstMonitor := range o.instMonitor {
		if remoteNode == o.localhost {
			continue
		}
		if remoteInstMonitor.State.IsDoing() {
			return true
		}
	}
	return false
}

func (o *imon) createPendingWithCancel() {
	o.pendingCtx, o.pendingCancel = context.WithCancel(o.ctx)
}

func (o *imon) createPendingWithDuration(duration time.Duration) {
	o.pendingCtx, o.pendingCancel = context.WithTimeout(o.ctx, duration)
}

func (o *imon) clearPending() {
	if o.pendingCancel != nil {
		o.pendingCancel()
		o.pendingCancel = nil
		o.pendingCtx = nil
	}
}

func (o *imon) loggerWithState() *zerolog.Logger {
	ctx := o.log.With()
	if o.state.GlobalExpect != instance.MonitorGlobalExpectZero {
		ctx.Str("global_expect", o.state.GlobalExpect.String())
	} else {
		ctx.Str("global_expect", "<zero>")
	}
	if o.state.LocalExpect != instance.MonitorLocalExpectZero {
		ctx.Str("local_expect", o.state.LocalExpect.String())
	} else {
		ctx.Str("local_expect", "<zero>")
	}
	stateLogger := ctx.Logger()
	return &stateLogger
}
