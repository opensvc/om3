package daemondata

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/durationlog"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	// Caller defines interface to implement for daemondata loop cmd processing
	// the function will hold the daemondata loop while running
	//    err := Caller.call(ctx, d)
	//    Caller.SetError(err)
	Caller interface {
		call(context.Context, *data) error
		SetError(error)
	}

	data struct {
		// previousRemoteInfo map[node] of remoteInfo from clusterData data just
		// after full message applied (used to publish detected diff on full message applied).
		previousRemoteInfo map[string]remoteInfo

		// clusterData is the live current data (after apply msg from patch or subscription)
		clusterData *msgbus.ClusterData

		pendingEvs    []event.Event // list of local events for peers (not yet committed to eventQueue)
		eventQueue    eventQueue    // queues of local data events for peers
		gen           uint64        // gen of local TNodeData
		hbMessageType string        // latest created hb message type
		localNode     string

		// cluster nodes from local cluster config, it is updated from
		// msgbus.ClusterConfigUpdated {NodesAdded, NodesRemoved}
		clusterNodes map[string]struct{}

		log *plog.Logger
		bus *pubsub.Bus
		sub *pubsub.Subscription

		// msgLocalGen hold the latest published msg gen for localhost
		msgLocalGen map[string]uint64
		hbSendQ     chan<- hbtype.Msg

		// hbMsgPatchLength holds the hb mode of cluster nodes:
		//
		// for local node: value is set during func (d *data) getHbMessage()
		// for peer:  value it set during func (d *data) onReceiveHbMsg
		hbMsgPatchLength map[string]int

		// hbMsgType track the hb message type of cluster nodes
		// - localhost associated value is changed during setNextMsgType
		// - other nodes associated value is changed during onReceiveHbMsg
		hbMsgType map[string]string

		// hbGens holds the cluster nodes gens
		//
		// values are used for the choice of next message type choice
		// - map[peer]map[string]uint64 it set from the received gens of peer
		// - map[localnode]map[string]uint64 it from local gen after successfull
		//   apply full, apply patch, or during commitPendingOps
		hbGens map[string]map[string]uint64

		// hbPatchMsgUpdated track last applied kind patch hb message
		// It is used to drop outdated patch messages
		hbPatchMsgUpdated map[string]time.Time

		// needMsg is set to true when a peer node doesn't know localnode current data gen
		// set to false after a hb message is created
		needMsg bool

		labelLocalNode pubsub.Label
	}

	gens       map[string]uint64
	eventQueue map[string][]event.Event

	// remoteInfo struct holds information about remote node used to publish diff on full message received
	remoteInfo struct {
		collectorUpdated  time.Time
		daemondataUpdated time.Time
		dnsUpdated        time.Time
		listenerUpdated   time.Time
		runnerImon        time.Time
		scheduler         time.Time

		nmonUpdated       time.Time
		nodeStats         node.Stats
		nodeStatus        node.Status
		nodeConfig        node.Config
		imonUpdated       map[string]time.Time
		instConfigUpdated map[string]time.Time
		instStatusUpdated map[string]time.Time
	}

	errC chan<- error
)

var (
	cmdDurationWarn = time.Second

	// propagationInterval is the minimum interval of:
	// - commit clusterData ops (update event queue, send local events to event.Event subscribers)
	// - pub applied changes from peers
	// - queueNewHbMsg (hb message type change, push msg to hb send queue)
	propagationInterval = 250 * time.Millisecond

	// subHbRefreshInterval is the minimum interval for update of: sub.hb
	subHbRefreshInterval = 100 * propagationInterval

	countRoutineInterval = 1 * time.Second

	ErrDrained = errors.New("drained command")

	labelFromPeer = pubsub.Label{"from", "peer"}

	onReceiveQueueOperationTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "opensvc_daemondata_receive_queue_operation_total",
			Help: "The total number of daemondata on receive queue operations",
		},
		[]string{"operation"})
)

func PropagationInterval() time.Duration {
	return propagationInterval
}

// run function loop on external events (op, hb) to updates data
// and queue hb message for hb sender
//
// the loop does following action in order
//
//			 1- on propagate ticker:
//			   commitPendingOps
//			   pubPeerDataChanges
//			   update sub hb stat on adaptive ticker (from 250ms to 25s)
//			   queueNewHbMsg when hb message is needed
//
//			 2- read hbrx message from queue -> onReceiveHbMsg
//			    apply ping
//			    or apply full
//			    or apply patch
//
//			3- process daemondata op commands from <-cmdC (chan <- caller)
//
//	       err := caller.call(ctx, d)
//	       caller.SetError(err)
//
//	     Note: client functions that send op caller to cmdC must use buffered
//	           channel to prevent daemondata loop hang during
//	           <client> <-channel-> <daemondata op processing>
//
//		During drain clusterData op commands (not yet processed by cmdC loop) will receive ErrDrained.
//	 => exposed caller clients can read the error channel to know if op commands succeed, failed or drained
//
//	 caller examples:
//
//	     type opGetX struct {
//	         errC
//	         resultC chan<- X
//	     }
//
//	     type opDoX struct {
//	         errC    // chan <- error
//	     }
//
//	     func (o opGetX) call(ctx context.Context, d *data) error {
//	         //
//	         return err
//	     }
//
//	    // client function
//	     func (t T) DoX() error {
//	          eC := make(chan err, 1)
//	          t.cmdC <- opDoX{errC: eC}
//	          return <- eC
//	     }
//
//	     // client function
//	     func (t T) GetX(...) X {
//	        eC := make(chan err, 1) // buffered channel to prevent hang
//	        resC := make(chan X, 1) // buffered channel to prevent hang
//	        t.cmdC <- opGetX{errC: eC, resultC: resC}
//	        if <-eC != nil {
//	            return X{}
//	        }
//	        // err is nil, we can read on resC
//	        return <- resC
//	     }
//
//	     // client function
//	     func (t T) GetXError(...) (x X, err error) {
//	        eC := make(chan err, 1) // buffered channel to prevent hang
//	        resC := make(chan X, 1) // buffered channel to prevent hang
//	        t.cmdC <- opGetY{errC: eC, resultC: resC}
//	        err = <- eC
//	        if err != nil {
//	            return
//	        }
//	        // err is nil, we can read on resC
//	        x <- resC
//	        return
//	     }
func (d *data) run(ctx context.Context, cmdC <-chan Caller, hbRecvQ <-chan *hbtype.Msg, drainDuration time.Duration) {
	d.log = plog.NewDefaultLogger().WithPrefix("daemon: data: ").Attr("pkg", "daemon/daemondata")
	d.log.Infof("starting")
	defer d.log.Infof("stopped")
	watchCmd := &durationlog.T{Log: *d.log}
	watchDurationCtx, watchDurationCancel := context.WithCancel(context.Background())
	defer watchDurationCancel()
	var beginCmd = make(chan interface{})
	var endCmd = make(chan bool)
	go func() {
		watchCmd.WarnExceeded(watchDurationCtx, beginCmd, endCmd, cmdDurationWarn, "data")
	}()

	propagationTicker := time.NewTicker(propagationInterval)
	defer propagationTicker.Stop()

	subHbRefreshAdaptiveInterval := propagationInterval
	subHbRefreshTicker := time.NewTicker(subHbRefreshAdaptiveInterval)

	defer subHbRefreshTicker.Stop()
	d.msgLocalGen = make(map[string]uint64)

	countRoutineTicker := time.NewTicker(countRoutineInterval)
	defer countRoutineTicker.Stop()

	doDrain := func() {
		d.log.Debugf("draining")
		defer d.log.Debugf("drained")

		tC := time.After(drainDuration)
		for {
			select {
			case <-hbRecvQ:
				// don't hang hbRecvQ writers
			case c := <-cmdC:
				c.SetError(ErrDrained)
			case <-tC:
				d.log.Debugf("drop clusterData cmds done")
				return
			}
		}
	}
	isCtxDone := func() bool {
		select {
		case <-ctx.Done():
			return true
		default:
			return false
		}
	}
	defer doDrain()
	for {
		select {
		case <-ctx.Done():
			return
		case <-propagationTicker.C:
			needMessage := d.commitPendingOps()
			if isCtxDone() {
				return
			}
			lgens := node.GenData.Get(d.localNode)
			if !needMessage && !gensEqual(d.msgLocalGen, *lgens) {
				needMessage = true
				if isCtxDone() {
					return
				}
				gens := make(map[string]uint64)
				for s, v := range *lgens {
					gens[s] = v
				}
				d.bus.Pub(&msgbus.NodeStatusGenUpdates{Node: d.localNode, Value: gens},
					d.labelLocalNode,
				)
			}
			if isCtxDone() {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-subHbRefreshTicker.C:
				d.setDaemonHeartbeat()
				d.log.Debugf("current hb msg mode %d", d.hbMsgPatchLength[d.localNode])
				needMessage = true
				if subHbRefreshAdaptiveInterval < subHbRefreshInterval {
					subHbRefreshAdaptiveInterval = 2 * subHbRefreshAdaptiveInterval
					subHbRefreshTicker.Reset(subHbRefreshAdaptiveInterval)
					d.log.Debugf("adapt interval for sub hb stat: %s", subHbRefreshAdaptiveInterval)
				}
			default:
			}
			select {
			case <-ctx.Done():
				return
			case <-countRoutineTicker.C:
				d.clusterData.Daemon.Routines = runtime.NumGoroutine()
			default:
			}
			if needMessage || d.needMsg {
				hbMsgType := d.hbMessageType
				if isCtxDone() {
					return
				}
				if err := d.queueNewHbMsg(ctx); err != nil {
					d.log.Errorf("queue hb message: %s %s", err)
				} else {
					d.needMsg = false
					if hbMsgType != d.hbMessageType {
						subHbRefreshAdaptiveInterval = propagationInterval
						d.log.Debugf("hb mg type changed, adapt interval for sub hb stat: %s", subHbRefreshAdaptiveInterval)
						subHbRefreshTicker.Reset(subHbRefreshAdaptiveInterval)
						hbMsgType = d.hbMessageType
					}
				}
			}
			propagationTicker.Reset(propagationInterval)
		case msg := <-hbRecvQ:
			if isCtxDone() {
				return
			}
			if _, ok := d.clusterNodes[msg.Nodename]; ok {
				d.onReceiveHbMsg(msg)
			} else {
				d.log.Warnf("drop rx message message: %s is not cluster member, cluster nodes: %+v", msg.Nodename, d.clusterNodes)
			}
		case cmd := <-cmdC:
			if c, ok := cmd.(Caller); ok {
				select {
				case <-ctx.Done():
					c.SetError(ctx.Err())
					return
				case beginCmd <- cmd:
				}
				err := c.call(ctx, d)
				c.SetError(err)
				select {
				case <-ctx.Done():
					return
				case endCmd <- true:
				}
			} else {
				d.log.Debugf("%s{...} is not a caller-interface cmd", reflect.TypeOf(cmd))
			}
		case i := <-d.sub.C:
			d.onSubEvent(i)
		}
	}
}

func (c errC) SetError(err error) {
	c <- err
}

func gensEqual(a, b gens) bool {
	if len(a) != len(b) {
		return false
	} else {
		for n, v := range a {
			if b[n] != v {
				return false
			}
		}
	}
	return true
}

// startSubscriptions subscribes to label local node messages that change the cluster data view
// or that must be forwarded to peers
func (d *data) startSubscriptions(qs pubsub.QueueSizer) {
	sub := d.bus.Sub("daemon.data", qs)
	sub.AddFilter(&msgbus.ClusterConfigUpdated{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.ClusterStatusUpdated{}, d.labelLocalNode)

	sub.AddFilter(&msgbus.DaemonCollectorUpdated{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.DaemonDataUpdated{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.DaemonDnsUpdated{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.DaemonHeartbeatUpdated{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.DaemonListenerUpdated{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.DaemonRunnerImonUpdated{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.DaemonSchedulerUpdated{}, d.labelLocalNode)

	sub.AddFilter(&msgbus.InstanceConfigDeleted{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.InstanceConfigFor{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.InstanceConfigUpdated{}, d.labelLocalNode)

	sub.AddFilter(&msgbus.InstanceMonitorDeleted{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.InstanceMonitorUpdated{}, d.labelLocalNode)

	sub.AddFilter(&msgbus.InstanceStatusUpdated{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.InstanceStatusDeleted{}, d.labelLocalNode)

	sub.AddFilter(&msgbus.NodeConfigUpdated{}, d.labelLocalNode)

	sub.AddFilter(&msgbus.NodeMonitorDeleted{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.NodeMonitorUpdated{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.NodeOsPathsUpdated{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.NodeStatsUpdated{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.NodeStatusUpdated{}, d.labelLocalNode)

	// need forward to peers
	sub.AddFilter(&msgbus.ObjectCreated{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.ObjectStatusDeleted{}, d.labelLocalNode)
	sub.AddFilter(&msgbus.ObjectStatusUpdated{}, d.labelLocalNode)
	sub.Start()
	d.sub = sub
}

func marshalEventData(v any) json.RawMessage {
	var b json.RawMessage
	b, _ = json.Marshal(v)
	return b
}

func (d *data) forwardEvent(i event.Kinder) {
	eventID++
	d.pendingEvs = append(d.pendingEvs, event.Event{
		Kind: i.Kind(),
		ID:   eventID,
		At:   time.Now(),
		Data: marshalEventData(i),
	})
}

// localEventMustBeForwarded returns true when local event i must be forwarded to peers
func localEventMustBeForwarded(i interface{}) bool {
	switch i.(type) {
	// daemon...
	case *msgbus.DaemonCollectorUpdated:
	case *msgbus.DaemonDataUpdated:
	case *msgbus.DaemonDnsUpdated:
	case *msgbus.DaemonHeartbeatUpdated:
	case *msgbus.DaemonListenerUpdated:
	case *msgbus.DaemonRunnerImonUpdated:
	case *msgbus.DaemonSchedulerUpdated:
	// instances...
	case *msgbus.InstanceConfigDeleted:
	case *msgbus.InstanceConfigFor:
	case *msgbus.InstanceConfigUpdated:
	case *msgbus.InstanceMonitorDeleted:
	case *msgbus.InstanceMonitorUpdated:
	case *msgbus.InstanceStatusUpdated:
	case *msgbus.InstanceStatusDeleted:
	// node...
	case *msgbus.NodeConfigUpdated:
	case *msgbus.NodeMonitorDeleted:
	case *msgbus.NodeMonitorUpdated:
	case *msgbus.NodeOsPathsUpdated:
	case *msgbus.NodeStatsUpdated:
	case *msgbus.NodeStatusUpdated:
	// object...
	case *msgbus.ObjectCreated:
	case *msgbus.ObjectStatusDeleted:
	default:
		return false
	}
	return true
}

func (d *data) updateClusterNodes(added, removed []string) {
	for _, s := range added {
		d.clusterNodes[s] = struct{}{}
	}
	for _, s := range removed {
		delete(d.clusterNodes, s)
	}
}

// onSubEvent is called on events emitted from localhost (has label node=localhost).
// It forwards event to peer (if localEventMustBeForwarded)
//
// when event is ClusterConfigUpdated: d.clusterNodes is refreshed and d.dropPeer
// is called for NodesRemoved
//
// finally event is applied to d.clusterData
func (d *data) onSubEvent(i interface{}) {
	if localEventMustBeForwarded(i) {
		if k, ok := i.(event.Kinder); ok {
			d.forwardEvent(k)
		}
	}

	if ev, ok := i.(*msgbus.ClusterConfigUpdated); ok {
		d.updateClusterNodes(ev.NodesAdded, ev.NodesRemoved)
		for _, s := range ev.NodesRemoved {
			d.log.Infof("removed cluster node => drop peer node %s data", s)
			d.dropPeer(s)
		}
	}

	if msg, ok := i.(pubsub.Messager); ok {
		d.clusterData.ApplyMessage(msg)
	}
}
