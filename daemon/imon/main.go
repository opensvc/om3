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
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
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
	Manager struct {
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

		// instConfig is the instance config value for path, it is updated on
		// ObjectStatusUpdated for path events where srcEvent is InstanceConfigUpdated.
		instConfig instance.Config

		// instStatus is the instance status value for path, it is updated on
		// ObjectStatusUpdated for path events where srcEvent is InstanceStatusDeleted
		// or InstanceStatusUpdated.
		instStatus map[string]instance.Status

		files filesManager

		// instMonitor tracks instance.Monitor for path on other nodes, iit is updated on
		// ObjectStatusUpdated for path events where srcEvent is InstanceMonitorDeleted
		// or InstanceMonitorUpdated from other nodes.
		instMonitor map[string]instance.Monitor

		nodeMonitor   map[string]node.Monitor
		nodeStats     map[string]node.Stats
		nodeStatus    map[string]node.Status
		readyDuration time.Duration
		scopeNodes    []string

		objStatus object.Status

		cancelReady context.CancelFunc
		localhost   string
		change      bool

		// statusQueued is true when a background status is running
		// TODO: need review
		statusQueued atomic.Bool

		// peerDrop stores which node needs a stonith before starting the local instance, if any
		peerDrop   string
		peerDropAt time.Time

		// needStatusQ is the status refresh pending request queue. A buffered
		// channel with 1 slot (If a status refresh is triggered twice (or more) before
		// the previous one starts, the extra status refresh is skipped). This happen
		// on sequential InstanceConfigUpdated events.
		needStatusQ chan priority.T

		// priors is the list of peer instance nodenames that need restarting before we can restart locally
		priors []string

		sub *pubsub.Subscription

		publisher pubsub.Publisher

		// waitConvergedOrchestrationMsg is a map indexed by nodename to latest waitConvergedOrchestrationMsg.
		// It is used while we are waiting for orchestration reached
		waitConvergedOrchestrationMsg map[string]string

		// orchestrationPending represents the ObjectOrchestrationEnd event for the
		// orchestration that has been accepted
		// It will be used to notify the end of orchestration
		orchestrationPending *msgbus.ObjectOrchestrationEnd

		// orchestrationAborted represents the ObjectOrchestrationEnd event for
		// an orchestration process that was prematurely aborted.
		orchestrationAborted *msgbus.ObjectOrchestrationEnd

		drainDuration time.Duration

		updateLimiter *rate.Limiter

		labelLocalhost pubsub.Label
		labelPath      pubsub.Label
		pubLabels      []pubsub.Label

		// delayDuration is the minimum duration between two imon orchestrate,
		// update.
		delayDuration time.Duration

		// delayOrchestrateEnabled a delayed orchestration has been asked
		// and will be run on next delayTimer hit.
		delayOrchestrateEnabled bool

		// delayPreUpdateEnabled a delayed pre update has been asked
		// and will be run on next delayTimer hit.
		delayPreUpdateEnabled bool

		// delayUpdateEnabled a delayed update has been asked
		// and will be run on next delayTimer hit.
		delayUpdateEnabled bool

		// delayTimer it the timer for the next delay task run:
		// onDelayTimer()
		delayTimer *time.Timer

		// delayTimerEnabled is true when the delay timer is already armed.
		// It is used during enableDelayTimer():
		// When false the delay timer is reset with delayDuration
		delayTimerEnabled bool

		// initialMonitorAction specifies the initial (stage 0) monitor action
		// for monitoring as defined by the MonitorAction type.
		// Its Value is created/refreshed during func initResourceMonitor.
		initialMonitorAction instance.MonitorAction

		// standbyResourceOrchestrate is the orchestrationResource for standby resources
		standbyResourceOrchestrate orchestrationResource

		// standbyResourceOrchestrate is the orchestrationResource for regular resources
		regularResourceOrchestrate orchestrationResource
	}

	filesManager struct {
		// fetched stores the resource files we fetched to avoid uneeded refetch
		fetched map[string]resource.File

		// attention stores a pending InstanceStatusUpdated event received while the fetch
		// manager was already processing an event. This serves as a flag to immediately
		// retrigger a new fetch cycle upon completion of the current one.
		attention *msgbus.InstanceStatusUpdated

		// fetching is true when the resource files fetch and ingest routine is
		// running
		fetching bool
	}

	// cmdOrchestrate can be used from post action go routines
	cmdOrchestrate struct {
		state    instance.MonitorState
		newState instance.MonitorState
	}

	// cmdResourceRestart is a structure representing a command to restart resources.
	// It can be used from imon goroutines to schedule a future resource restart
	// handled during imon main loop.
	// rids is a slice of resource IDs to restart.
	// standby indicates whether the resources should restart in standby mode.
	cmdResourceRestart struct {
		rids    []string
		standby bool
	}

	cmdFetchDone struct {
		Files resource.Files
	}

	Factory struct {
		DrainDuration time.Duration
		SubQS         pubsub.QueueSizer
		DelayDuration time.Duration
	}
)

// Start creates a new imon and starts worker goroutine to manage local instance monitor
func (f Factory) Start(parent context.Context, p naming.Path, nodes []string) error {
	return start(parent, f.SubQS, p, nodes, f.DelayDuration, f.DrainDuration)
}

var (
	// defaultReadyDuration is pickup from daemonenv.ReadyDuration. It should not be
	// changed without verify possible impacts on cluster split detection.
	defaultReadyDuration = daemonenv.ReadyDuration

	// updateRate is the limit rate for imon publish updates per second
	// when orchestration loop occur on an object, too many events/commands may block
	// databus or event bus. We must prevent such situations
	// TODO: no longer used, replaced by delayTimer
	updateRate rate.Limit = 25
)

// start launch goroutine imon worker for a local instance state
func start(parent context.Context, qs pubsub.QueueSizer, p naming.Path, nodes []string, delayDuration, drainDuration time.Duration) error {
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
	t := &Manager{
		state:         state,
		previousState: previousState,
		path:          p,
		id:            id,
		ctx:           ctx,
		cancel:        cancel,
		cmdC:          make(chan any),
		databus:       databus,
		publisher:     pubsub.PubFromContext(ctx),
		files: filesManager{
			fetched: make(map[string]resource.File),
		},
		instStatus:    make(map[string]instance.Status),
		instMonitor:   make(map[string]instance.Monitor),
		nodeMonitor:   make(map[string]node.Monitor),
		nodeStats:     make(map[string]node.Stats),
		nodeStatus:    make(map[string]node.Status),
		priors:        make([]string, 0),
		localhost:     localhost,
		scopeNodes:    nodes,
		change:        true,
		readyDuration: defaultReadyDuration,

		waitConvergedOrchestrationMsg: make(map[string]string),

		drainDuration: drainDuration,

		updateLimiter: rate.NewLimiter(updateRate, int(updateRate)),
		delayDuration: delayDuration,

		labelLocalhost: pubsub.Label{"node", localhost},
		labelPath:      pubsub.Label{"path", id},
		pubLabels: []pubsub.Label{
			{"namespace", p.Namespace},
			{"path", id},
			{"node", localhost},
		},

		needStatusQ: make(chan priority.T, 1),
	}

	t.log = t.newLogger(uuid.Nil)
	t.regularResourceOrchestrate.log = t.newResourceLogger("regular resource")
	t.standbyResourceOrchestrate.log = t.newResourceLogger("standby resource")

	t.startSubscriptions(qs)

	go func() {
		t.worker(nodes)
	}()

	return nil
}

func (t *Manager) newResourceLogger(s string) *plog.Logger {
	return naming.LogWithPath(plog.NewDefaultLogger(), t.path).
		Attr("pkg", "daemon/imon").
		WithPrefix(fmt.Sprintf("daemon: imon: %s: %s: ", t.path, s))
}

func (t *Manager) newLogger(i uuid.UUID) *plog.Logger {
	return naming.LogWithPath(plog.NewDefaultLogger(), t.path).
		Attr("pkg", "daemon/imon").
		Attr("orchestration_id", i.String()).
		WithPrefix(fmt.Sprintf("daemon: imon: %s: ", t.path.String()))
}

func (t *Manager) startSubscriptions(qs pubsub.QueueSizer) {
	sub := pubsub.SubFromContext(t.ctx, "daemon.imon "+t.id, qs)
	sub.AddFilter(&msgbus.NodeConfigUpdated{}, t.labelLocalhost)
	sub.AddFilter(&msgbus.NodeMonitorUpdated{})
	sub.AddFilter(&msgbus.NodeRejoin{}, t.labelLocalhost)
	sub.AddFilter(&msgbus.NodeStatusUpdated{})
	sub.AddFilter(&msgbus.NodeStatsUpdated{})
	sub.AddFilter(&msgbus.ObjectStatusUpdated{}, t.labelPath)
	sub.AddFilter(&msgbus.ProgressInstanceMonitor{}, t.labelPath)
	sub.AddFilter(&msgbus.SetInstanceMonitor{}, t.labelPath)
	sub.Start()
	t.sub = sub
}

// worker watch for local imon updates
func (t *Manager) worker(initialNodes []string) {
	defer t.log.Debugf("worker stopped")

	// runStatus and requestStatusRefresh will need instance config Priority
	if iConfig := instance.ConfigData.GetByPathAndNode(t.path, t.localhost); iConfig != nil {
		t.instConfig = *iConfig
		t.scopeNodes = append([]string{}, t.instConfig.Scope...)
	} else {
		t.log.Infof("return on empty instance config")
		return
	}

	// Initiate a CRM status refresh first, this will update our instance status cache
	// as soon as possible.
	// runStatus => publish instance status update
	//   => data update (so available from next GetInstanceStatus)
	//   => omon update with srcEvent: instance status update (we watch omon updates)
	if err := t.runStatus(t.instConfig.Priority); err != nil {
		t.log.Errorf("error during initial crm status: %s", err)
	}

	t.statusRunner()

	if t.bootAble() {
		t.ensureBooted()
	}

	// Populate caches (published messages before subscription startup are lost)
	for _, v := range node.StatusData.GetAll() {
		t.nodeStatus[v.Node] = *v.Value
	}
	for _, v := range node.StatsData.GetAll() {
		t.nodeStats[v.Node] = *v.Value
	}
	for _, v := range node.MonitorData.GetAll() {
		t.nodeMonitor[v.Node] = *v.Value
	}
	for n, v := range instance.MonitorData.GetByPath(t.path) {
		if n == t.localhost {
			// skip localhost, t.instMonitor tracks t.path instance monitor of peers
			continue
		}
		t.instMonitor[n] = *v
	}
	for n, v := range instance.StatusData.GetByPath(t.path) {
		t.instStatus[n] = *v
	}

	t.delayTimer = time.NewTimer(time.Second)
	if !t.delayTimer.Stop() {
		<-t.delayTimer.C
	}

	t.initRelationAvailStatus()
	t.initResourceMonitor()
	t.updateIsLeader()
	t.updateIfChange()

	defer func() {
		go func() {
			err := t.sub.Stop()
			if err != nil && !errors.Is(err, context.Canceled) {
				t.log.Errorf("subscription stop: %s", err)
			}
		}()
		instance.StatusData.Unset(t.path, t.localhost)
		t.publisher.Pub(&msgbus.InstanceStatusDeleted{Path: t.path, Node: t.localhost}, t.pubLabels...)
		instance.MonitorData.Unset(t.path, t.localhost)

		// a last chance to publish any pending aborted orchestration
		t.publishOrchestrationAborted()

		var pending msgbus.ObjectOrchestrationEnd
		if t.orchestrationPending != nil {
			pending = *t.orchestrationPending
		}
		t.publisher.Pub(&msgbus.InstanceMonitorDeleted{Path: t.path, Node: t.localhost, OrchestrationEnd: pending}, t.pubLabels...)

		go func() {
			tC := time.After(t.drainDuration)
			for {
				select {
				case <-tC:
					return
				case <-t.cmdC:
				}
			}
		}()
	}()
	t.log.Debugf("started")
	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-t.sub.C:
			if t.ctx.Err() != nil {
				t.log.Debugf("skipping event due to canceled context")
				return
			}
			switch c := i.(type) {
			case *msgbus.InstanceStatusDeleted:
				t.onInstanceStatusDeleted(c)
			case *msgbus.InstanceStatusUpdated:
				t.onRelationInstanceStatusUpdated(c)
			case *msgbus.ObjectStatusDeleted:
				t.onObjectStatusDeleted(c)
			case *msgbus.ObjectStatusUpdated:
				t.onObjectStatusUpdated(c)
			case *msgbus.ProgressInstanceMonitor:
				t.onProgressInstanceMonitor(c)
			case *msgbus.SetInstanceMonitor:
				t.onSetInstanceMonitor(c)
			case *msgbus.NodeConfigUpdated:
				t.onNodeConfigUpdated(c)
			case *msgbus.NodeMonitorUpdated:
				t.onNodeMonitorUpdated(c)
			case *msgbus.NodeRejoin:
				t.onNodeRejoin(c)
			case *msgbus.NodeStatusUpdated:
				t.onNodeStatusUpdated(c)
			case *msgbus.NodeStatsUpdated:
				t.onNodeStatsUpdated(c)
			}
		case i := <-t.cmdC:
			if t.ctx.Err() != nil {
				t.log.Debugf("skipping cmd due to canceled context")
				return
			}
			switch c := i.(type) {
			case cmdOrchestrate:
				t.needOrchestrate(c)
			case cmdResourceRestart:
				t.resourceRestart(c.rids, c.standby)
			case cmdFetchDone:
				t.onFetchDone(c)
			}
		case <-t.delayTimer.C:
			t.onDelayTimer()
		}
	}
}

// ensureBooted runs the bot action on not yet booted object
func (t *Manager) ensureBooted() {
	instanceLastBootID := lastBootID(t.path)
	nodeLastBootID := bootid.Get()
	if instanceLastBootID == "" {
		// no last instance boot file, create it
		t.log.Infof("set last object boot id")
		if err := updateLastBootID(t.path, nodeLastBootID); err != nil {
			t.log.Errorf("can't update instance last boot id file: %s", err)
		}
	} else if instanceLastBootID != bootid.Get() {
		// last instance boot id differ from current node boot id
		// try boot and refresh last instance boot id if succeed
		t.log.Infof("need boot (node boot id differ from last object boot id)")
		t.transitionTo(instance.MonitorStateBootProgress)
		if err := t.queueBoot(); err == nil {
			t.log.Infof("set last object boot id")
			if err := updateLastBootID(t.path, nodeLastBootID); err != nil {
				t.log.Errorf("can't update instance last boot id file: %s", err)
			}
			t.transitionTo(instance.MonitorStateBootSuccess)
			t.transitionTo(instance.MonitorStateIdle)
		} else {
			// boot failed, next daemon restart will retry boot
			t.transitionTo(instance.MonitorStateBootFailed)
		}
	}
}

func (t *Manager) update() {
	select {
	case <-t.ctx.Done():
		return
	default:
	}
	if err := t.updateLimiter.Wait(t.ctx); err != nil {
		return
	}

	t.state.UpdatedAt = time.Now()
	newValue := t.state

	instance.MonitorData.Set(t.path, t.localhost, newValue.DeepCopy())
	t.publisher.Pub(&msgbus.InstanceMonitorUpdated{Path: t.path, Node: t.localhost, Value: newValue}, t.pubLabels...)
}

func (t *Manager) transitionTo(newState instance.MonitorState) {
	t.change = true
	t.state.State = newState
	t.updateIfChange()
}

// updateIfChange log updates and publish new state value when changed
func (t *Manager) updateIfChange() {
	select {
	case <-t.ctx.Done():
		return
	default:
	}
	if t.state.OrchestrationID == uuid.Nil &&
		t.state.State == instance.MonitorStateIdle &&
		t.state.LocalExpect != instance.MonitorLocalExpectStarted &&
		t.instStatus[t.localhost].Avail.Is(status.Up) {
		if t.instStatus[t.localhost].UpdatedAt.After(t.state.LocalExpectUpdatedAt) {
			// no orchestration, state is idle, avail is up, monitor is not yet enabled
			// and instance status is more recent than the `LocalExpectUpdatedAt`.
			//
			// Must wait for status UpdatedAt After localExpectUpdatedAt to avoid the following scenario:
			//   time 0 daemon: imon: obj1 knows local instance avail as UP
			//   11:50:17.982808 om[1849638]: instance: obj1 >>> do stop [om obj1 stop --rid app#1] (origin user,...
			//   11:50:17.984207 om[1846732]: daemon: imon: obj1 progress instance monitor state idle -> stopping
			//   11:50:17.984262 om[1846732]: daemon: imon: obj1 user is stopping some instance resources: disable resource restart and monitoring ✅
			//                            =>  daemon: imon: obj1 change local expect started -> none
			//                            =>  daemon: imon: obj1 change state idle -> stopping
			//   11:50:18.444875 om[1849638]: instance: obj1 app#1: run: ...
			//                            =>  daemon: istat: obj1 change avail up -> warn
			//                                instance avail is warn from istat, but imon doesn't know it yet.
			//   11:50:19.689713 om[1846732]: daemon: imon: obj1 progress instance monitor state stopping -> idle
			//   11:50:19.689771 om[1846732]: daemon: imon: obj1 local instance is up and idle: enable resource restart and monitoring
			//  						     🐞unexpected local expect started rearmed, we should have waited for fresher instance status
			// 			 				     before enable resource restart and monitoring
			//   11:50:19.689814 om[1846732]: daemon: imon: obj1 change local expect none -> started
			//   11:50:19.689857 om[1846732]: daemon: imon: obj1 change state stopping -> idle
			//   11:50:19.690198 om[1846732]: daemon: imon: obj1 ObjectStatusUpdated node1 from InstanceStatusUpdated on node1 update instance status with avail warn
			//							    ✅now, the instance status updated at is newer than the local expect updated at
			t.enableMonitor("local instance is up and idle")
		} else {
			t.log.Debugf("wait for fresher instance status before enable enable resource restart and monitoring")
		}
	}
	if !t.change {
		return
	}
	t.change = false
	now := time.Now()
	previousVal := t.previousState
	newVal := t.state
	if newVal.GlobalExpect != previousVal.GlobalExpect {
		// Don't update GlobalExpectUpdated here
		// GlobalExpectUpdated is updated only during cmdSetInstanceMonitorClient and
		// its value is used for convergeGlobalExpectFromRemote
		t.loggerWithState().Infof("change global expect %s -> %s", previousVal.GlobalExpect, newVal.GlobalExpect)
	}
	if newVal.LocalExpect != previousVal.LocalExpect {
		t.state.LocalExpectUpdatedAt = now
		t.loggerWithState().Infof("change local expect %s -> %s", previousVal.LocalExpect, newVal.LocalExpect)
	}
	if newVal.State != previousVal.State {
		t.state.StateUpdatedAt = now
		t.loggerWithState().Infof("change state %s -> %s", previousVal.State, newVal.State)
	}
	if newVal.IsLeader != previousVal.IsLeader {
		t.loggerWithState().Infof("change leader %t -> %t", previousVal.IsLeader, newVal.IsLeader)
	}
	if newVal.IsHALeader != previousVal.IsHALeader {
		t.loggerWithState().Infof("change ha leader %t -> %t", previousVal.IsHALeader, newVal.IsHALeader)
	}
	t.previousState = t.state
	t.update()
}

func (t *Manager) hasOtherNodeActing() bool {
	for remoteNode, remoteInstMonitor := range t.instMonitor {
		if remoteNode == t.localhost {
			continue
		}
		if remoteInstMonitor.State.IsDoing() {
			return true
		}
	}
	return false
}

func (t *Manager) createPendingWithDuration(duration time.Duration) {
	t.log.Debugf("create new pending context with duration %s", duration)
	t.pendingCtx, t.pendingCancel = context.WithTimeout(t.ctx, duration)
}

func (t *Manager) clearPending() {
	if t.pendingCancel != nil {
		t.log.Debugf("clear pending context")
		t.pendingCancel()
		t.pendingCancel = nil
		t.pendingCtx = nil
	}
}

func (t *Manager) loggerWithState() *plog.Logger {
	return t.log.
		Attr("imon_global_expect", t.state.GlobalExpect.String()).
		Attr("imon_local_expect", t.state.LocalExpect.String()).
		Attr("imon_state", t.state.State.String())
}

func lastBootIDFile(p naming.Path) string {
	if p.Namespace != naming.NsRoot && p.Namespace != "" {
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

func (t *Manager) bootAble() bool {
	if t.instConfig.ActorConfig == nil {
		return false
	}
	if t.instConfig.IsDisabled {
		// ensures disabled instances are not erroneously booted, aligning
		// behavior with the intended configuration.
		return false
	}
	return true
}
