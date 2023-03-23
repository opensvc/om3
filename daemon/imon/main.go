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
	"errors"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/daemonenv"
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

		drainDuration time.Duration

		updateLimiter *rate.Limiter

		labelLocalhost pubsub.Label
		labelPath      pubsub.Label
	}

	// cmdOrchestrate can be used from post action go routines
	cmdOrchestrate struct {
		state    instance.MonitorState
		newState instance.MonitorState
	}

	Factory struct {
		DrainDuration time.Duration
	}
)

// Start creates a new imon and starts worker goroutine to manage local instance monitor
func (f Factory) Start(parent context.Context, p path.T, nodes []string) error {
	return start(parent, p, nodes, f.DrainDuration)
}

var (
	// defaultReadyDuration is pickup from daemonenv.ReadyDuration. It should not be
	// changed without verify possible impacts on cluster split detection.
	defaultReadyDuration = daemonenv.ReadyDuration

	// updateRate is the limit rate for imon publish updates per second
	// when orchestration loop occur on an object, too many events/commands may block
	// databus or event bus. We must prevent such situations
	updateRate rate.Limit = 25
)

// start launch goroutine imon worker for a local instance state
func start(parent context.Context, p path.T, nodes []string, drainDuration time.Duration) error {
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

	localhost := hostname.Hostname()

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
		localhost:     localhost,
		scopeNodes:    nodes,
		change:        true,
		readyDuration: defaultReadyDuration,

		waitConvergedOrchestrationMsg: make(map[string]string),

		drainDuration: drainDuration,

		updateLimiter: rate.NewLimiter(updateRate, int(updateRate)),

		nodeMonitor: make(map[string]node.Monitor),

		labelLocalhost: pubsub.Label{"node", localhost},
		labelPath:      pubsub.Label{"path", id},
	}

	o.startSubscriptions()
	o.nodeStatus = databus.GetNodeStatusMap()
	o.nodeStats = databus.GetNodeStatsMap()
	o.instMonitor = databus.GetInstanceMonitorMap(o.path)
	o.instConfig = databus.GetInstanceConfig(o.path, o.localhost)
	o.initResourceMonitor()

	go func() {
		o.worker(nodes)
	}()

	return nil
}

func (o *imon) startSubscriptions() {
	sub := o.pubsubBus.Sub(o.id + " imon")
	sub.AddFilter(msgbus.ObjectStatusUpdated{}, o.labelPath)
	sub.AddFilter(msgbus.ProgressInstanceMonitor{}, o.labelPath)
	sub.AddFilter(msgbus.SetInstanceMonitor{}, o.labelPath)
	sub.AddFilter(msgbus.NodeConfigUpdated{}, o.labelLocalhost)
	sub.AddFilter(msgbus.NodeMonitorUpdated{})
	sub.AddFilter(msgbus.NodeStatusUpdated{})
	sub.AddFilter(msgbus.NodeStatsUpdated{})
	sub.Start()
	o.sub = sub
}

// worker watch for local imon updates
func (o *imon) worker(initialNodes []string) {
	defer o.log.Debug().Msg("done")

	// Initiate crmStatus fisrt, this will update our instance status cache
	// as soon as possible.
	// crmStatus => publish instance status update
	//   => data update (so available from next GetInstanceStatus)
	//   => omon update with srcEvent: instance status update (we watch omon updates)
	if err := o.crmStatus(); err != nil {
		o.log.Error().Err(err).Msg("error during initial crm status")
	}

	for _, initialNode := range initialNodes {
		o.instStatus[initialNode] = o.databus.GetInstanceStatus(o.path, initialNode)
	}
	for _, v := range o.sub.GetLasts(msgbus.NodeMonitorUpdated{}) {
		nodeMonitor := v.(msgbus.NodeMonitorUpdated)
		o.nodeMonitor[nodeMonitor.Node] = *nodeMonitor.Value.DeepCopy()
	}

	o.updateIfChange()
	defer func() {
		go func() {
			err := o.sub.Stop()
			if err != nil && !errors.Is(err, context.Canceled) {
				o.log.Error().Err(err).Msg("subscription stop")
			}
		}()
		go func() {
			instance.MonitorData.Unset(o.path, o.localhost)
			o.pubsubBus.Pub(msgbus.InstanceMonitorDeleted{Path: o.path, Node: o.localhost},
				o.labelPath,
				o.labelLocalhost,
			)
		}()
		go func() {
			tC := time.After(o.drainDuration)
			for {
				select {
				case <-tC:
					return
				case <-o.cmdC:
				}
			}
		}()
	}()
	o.log.Debug().Msg("started")
	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.sub.C:
			select {
			case <-o.ctx.Done():
				return
			default:
			}
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
			select {
			case <-o.ctx.Done():
				return
			default:
			}
			switch c := i.(type) {
			case cmdOrchestrate:
				o.needOrchestrate(c)
			}
		}
	}
}

func (o *imon) update() {
	select {
	case <-o.ctx.Done():
		return
	default:
	}
	if err := o.updateLimiter.Wait(o.ctx); err != nil {
		return
	}

	o.state.UpdatedAt = time.Now()
	newValue := o.state

	instance.MonitorData.Set(o.path, o.localhost, newValue.DeepCopy())
	o.pubsubBus.Pub(msgbus.InstanceMonitorUpdated{Path: o.path, Node: o.localhost, Value: newValue},
		o.labelPath,
		o.labelLocalhost,
	)
}

func (o *imon) transitionTo(newState instance.MonitorState) {
	o.change = true
	o.state.State = newState
	o.updateIfChange()
}

// updateIfChange log updates and publish new state value when changed
func (o *imon) updateIfChange() {
	select {
	case <-o.ctx.Done():
		return
	default:
	}
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
