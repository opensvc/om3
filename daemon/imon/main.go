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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/bootid"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	imon struct {
		state         instance.Monitor
		previousState instance.Monitor

		path    naming.Path
		id      string
		ctx     context.Context
		cancel  context.CancelFunc
		cmdC    chan any
		databus *daemondata.T
		log     *plog.Logger

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		// updated data from object status update srcEvent
		instConfig    instance.Config
		instStatus    map[string]instance.Status
		instMonitor   map[string]instance.Monitor
		nodeMonitor   map[string]node.Monitor
		nodeStats     map[string]node.Stats
		nodeStatus    map[string]node.Status
		readyDuration time.Duration
		scopeNodes    []string

		objStatus object.Status

		cancelReady context.CancelFunc
		localhost   string
		change      bool

		sub *pubsub.Subscription

		pubsubBus *pubsub.Bus

		// waitConvergedOrchestrationMsg is a map indexed by nodename to latest waitConvergedOrchestrationMsg.
		// It is used while we are waiting for orchestration reached
		waitConvergedOrchestrationMsg map[string]string

		acceptedOrchestrationId uuid.UUID

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
func (f Factory) Start(parent context.Context, p naming.Path, nodes []string) error {
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
func start(parent context.Context, p naming.Path, nodes []string, drainDuration time.Duration) error {
	ctx, cancel := context.WithCancel(parent)
	id := p.String()

	previousState := instance.Monitor{
		LocalExpect:    instance.MonitorLocalExpectNone,
		GlobalExpect:   instance.MonitorGlobalExpectNone,
		State:          instance.MonitorStateIdle,
		Resources:      make(instance.ResourceMonitors, 0),
		Children:       make(map[string]status.T),
		Parents:        make(map[string]status.T),
		StateUpdatedAt: time.Now(),
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
		log:           plog.NewDefaultLogger().Attr("pkg", "daemon/imon").Attr("obj_path", p.String()).WithPrefix(fmt.Sprintf("daemon: imon: %s: ", p)),
		instStatus:    make(map[string]instance.Status),
		instMonitor:   make(map[string]instance.Monitor),
		nodeMonitor:   make(map[string]node.Monitor),
		nodeStats:     make(map[string]node.Stats),
		nodeStatus:    make(map[string]node.Status),
		localhost:     localhost,
		scopeNodes:    nodes,
		change:        true,
		readyDuration: defaultReadyDuration,

		waitConvergedOrchestrationMsg: make(map[string]string),

		drainDuration: drainDuration,

		updateLimiter: rate.NewLimiter(updateRate, int(updateRate)),

		labelLocalhost: pubsub.Label{"node", localhost},
		labelPath:      pubsub.Label{"path", id},
	}

	o.startSubscriptions()

	go func() {
		o.worker(nodes)
	}()

	return nil
}

func (o *imon) startSubscriptions() {
	sub := o.pubsubBus.Sub(o.id + " imon")
	sub.AddFilter(&msgbus.NodeConfigUpdated{}, o.labelLocalhost)
	sub.AddFilter(&msgbus.NodeMonitorUpdated{})
	sub.AddFilter(&msgbus.NodeRejoin{}, o.labelLocalhost)
	sub.AddFilter(&msgbus.NodeStatusUpdated{})
	sub.AddFilter(&msgbus.NodeStatsUpdated{})
	sub.AddFilter(&msgbus.ObjectStatusUpdated{}, o.labelPath)
	sub.AddFilter(&msgbus.ProgressInstanceMonitor{}, o.labelPath)
	sub.AddFilter(&msgbus.SetInstanceMonitor{}, o.labelPath)
	sub.Start()
	o.sub = sub
}

// worker watch for local imon updates
func (o *imon) worker(initialNodes []string) {
	defer o.log.Debugf("worker started")

	// Initiate crmStatus first, this will update our instance status cache
	// as soon as possible.
	// crmStatus => publish instance status update
	//   => data update (so available from next GetInstanceStatus)
	//   => omon update with srcEvent: instance status update (we watch omon updates)
	if err := o.crmStatus(); err != nil {
		o.log.Errorf("error during initial crm status: %s", err)
	}

	if o.bootAble() {
		o.ensureBooted()
	}

	// Populate caches (published messages before subscription startup are lost)
	for _, v := range node.StatusData.GetAll() {
		o.nodeStatus[v.Node] = *v.Value
	}
	for _, v := range node.StatsData.GetAll() {
		o.nodeStats[v.Node] = *v.Value
	}
	for _, v := range node.MonitorData.GetAll() {
		o.nodeMonitor[v.Node] = *v.Value
	}
	if iConfig := instance.ConfigData.Get(o.path, o.localhost); iConfig != nil {
		o.instConfig = *iConfig
		o.scopeNodes = append([]string{}, o.instConfig.Scope...)
	}
	for n, v := range instance.MonitorData.GetByPath(o.path) {
		if n == o.localhost {
			o.log.Warnf("bug: on init, instance.MonitorData[localhost] should be empty")
			continue
		}
		o.instMonitor[n] = *v
	}
	for n, v := range instance.StatusData.GetByPath(o.path) {
		o.instStatus[n] = *v
	}

	o.initRelationAvailStatus()
	o.initResourceMonitor()
	o.updateIsLeader()
	o.updateIfChange()

	defer func() {
		go func() {
			err := o.sub.Stop()
			if err != nil && !errors.Is(err, context.Canceled) {
				o.log.Errorf("subscription stop: %s", err)
			}
		}()
		go func() {
			instance.MonitorData.Unset(o.path, o.localhost)
			o.pubsubBus.Pub(&msgbus.InstanceMonitorDeleted{Path: o.path, Node: o.localhost},
				o.labelPath,
				o.labelLocalhost,
			)
		}()
		go func() {
			instance.StatusData.Unset(o.path, o.localhost)
			o.pubsubBus.Pub(&msgbus.InstanceStatusDeleted{Path: o.path, Node: o.localhost},
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
	o.log.Debugf("started")
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
			case *msgbus.InstanceStatusDeleted:
				o.onInstanceStatusDeleted(c)
			case *msgbus.InstanceStatusUpdated:
				o.onRelationInstanceStatusUpdated(c)
			case *msgbus.ObjectStatusDeleted:
				o.onObjectStatusDeleted(c)
			case *msgbus.ObjectStatusUpdated:
				o.onObjectStatusUpdated(c)
			case *msgbus.ProgressInstanceMonitor:
				o.onProgressInstanceMonitor(c)
			case *msgbus.SetInstanceMonitor:
				o.onSetInstanceMonitor(c)
			case *msgbus.NodeConfigUpdated:
				o.onNodeConfigUpdated(c)
			case *msgbus.NodeMonitorUpdated:
				o.onNodeMonitorUpdated(c)
			case *msgbus.NodeRejoin:
				o.onNodeRejoin(c)
			case *msgbus.NodeStatusUpdated:
				o.onNodeStatusUpdated(c)
			case *msgbus.NodeStatsUpdated:
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

// ensureBooted runs the bot action on not yet booted object
func (o *imon) ensureBooted() {
	instanceLastBootID := lastBootID(o.path)
	nodeLastBootID := bootid.Get()
	if instanceLastBootID == "" {
		// no last instance boot file, create it
		o.log.Infof("set last object boot id")
		if err := updateLastBootID(o.path, nodeLastBootID); err != nil {
			o.log.Errorf("can't update instance last boot id file: %s", err)
		}
	} else if instanceLastBootID != bootid.Get() {
		// last instance boot id differ from current node boot id
		// try boot and refresh last instance boot id if succeed
		o.log.Infof("need boot (node boot id differ from last object boot id)")
		o.transitionTo(instance.MonitorStateBooting)
		if err := o.crmBoot(); err == nil {
			o.log.Infof("set last object boot id")
			if err := updateLastBootID(o.path, nodeLastBootID); err != nil {
				o.log.Errorf("can't update instance last boot id file: %s", err)
			}
			o.transitionTo(instance.MonitorStateBooted)
			o.transitionTo(instance.MonitorStateIdle)
		} else {
			// boot failed, next daemon restart will retry boot
			o.transitionTo(instance.MonitorStateBootFailed)
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
	o.pubsubBus.Pub(&msgbus.InstanceMonitorUpdated{Path: o.path, Node: o.localhost, Value: newValue},
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
		o.loggerWithState().Infof("change global expect %s -> %s", previousVal.GlobalExpect, newVal.GlobalExpect)
	}
	if newVal.LocalExpect != previousVal.LocalExpect {
		o.state.LocalExpectUpdatedAt = now
		o.loggerWithState().Infof("change local expect %s -> %s", previousVal.LocalExpect, newVal.LocalExpect)
	}
	if newVal.State != previousVal.State {
		o.state.StateUpdatedAt = now
		o.loggerWithState().Infof("change state %s -> %s", previousVal.State, newVal.State)
	}
	if newVal.IsLeader != previousVal.IsLeader {
		o.loggerWithState().Infof("change leader %t -> %t", previousVal.IsLeader, newVal.IsLeader)
	}
	if newVal.IsHALeader != previousVal.IsHALeader {
		o.loggerWithState().Infof("change ha leader %t -> %t", previousVal.IsHALeader, newVal.IsHALeader)
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

func (o *imon) createPendingWithDuration(duration time.Duration) {
	o.log.Debugf("create new pending context with duration %s", duration)
	o.pendingCtx, o.pendingCancel = context.WithTimeout(o.ctx, duration)
}

func (o *imon) clearPending() {
	if o.pendingCancel != nil {
		o.log.Debugf("clear pending context")
		o.pendingCancel()
		o.pendingCancel = nil
		o.pendingCtx = nil
	}
}

func (o *imon) loggerWithState() *plog.Logger {
	return o.log.
		Attr("imon_global_expect", o.state.GlobalExpect.String()).
		Attr("imon_local_expect", o.state.LocalExpect.String()).
		Attr("imon_state", o.state.State.String())
}

func lastBootIDFile(p naming.Path) string {
	if p.Namespace != "root" && p.Namespace != "" {
		return filepath.Join(rawconfig.Paths.Var, "namespaces", p.String(), "last_boot_id")
	} else {
		return filepath.Join(rawconfig.Paths.Var, p.Kind.String(), p.Name, "last_boot_id")
	}
}

func lastBootID(p naming.Path) string {
	if b, err := os.ReadFile(lastBootIDFile(p)); err != nil {
		return ""
	} else {
		return string(b)
	}
}

func updateLastBootID(p naming.Path, s string) error {
	return os.WriteFile(lastBootIDFile(p), []byte(s), 0644)
}

func (o *imon) bootAble() bool {
	switch o.path.Kind {
	case naming.KindSvc:
		return true
	case naming.KindVol:
		return true
	default:
		return false
	}
}
