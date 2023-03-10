package daemondata

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/durationlog"
	"github.com/opensvc/om3/util/jsondelta"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	// caller defines interface to implement for daemondata loop cmd processing
	// the function will hold the daemondata loop while running
	//    err := caller.call(ctx, d)
	//    caller.SetError(err)
	caller interface {
		call(context.Context, *data) error
		SetError(error)
	}

	data struct {
		// previousRemoteInfo map[node] of remoteInfo from pending data just
		// after commit, it is used to publish diff for other nodes
		previousRemoteInfo map[string]remoteInfo

		// pending is the live current data (after apply patch, commit local pendingOps)
		pending *cluster.Data

		pendingOps    []jsondelta.Operation // local data pending operations not yet in patchQueue
		patchQueue    patchQueue            // local data patch queue for remotes
		gen           uint64                // gen of local TNodeData
		hbMessageType string                // latest created hb message type
		localNode     string

		// cluster nodes from local cluster config
		clusterNodes map[string]struct{}

		// statCount is a map[<stat id>] to track number of <id> calls
		statCount map[int]uint64
		log       zerolog.Logger
		bus       *pubsub.Bus
		sub       *pubsub.Subscription

		// msgLocalGen hold the latest published msg gen for localhost
		msgLocalGen map[string]uint64
		hbSendQ     chan<- hbtype.Msg

		// hbMsgMode holds the hb mode of cluster nodes:
		//
		// for local node: value is set during func (d *data) getHbMessage()
		// for peer:  value it set during func (d *data) onReceiveHbMsg
		// It has same value as hbMsgType, except when type is patch where it represents size of patch queue
		hbMsgMode map[string]string

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
	patchQueue map[string]jsondelta.Patch

	// remoteInfo struct holds information about remote node used to publish diff
	remoteInfo struct {
		nmonUpdated       time.Time
		nodeStats         node.Stats
		nodeStatus        node.Status
		nodeConfig        node.Config
		imonUpdated       map[string]time.Time
		instConfigUpdated map[string]time.Time
		instStatusUpdated map[string]time.Time
		gen               uint64
	}

	errC chan<- error
)

var (
	cmdDurationWarn = time.Second

	// propagationInterval is the minimum interval of:
	// - commit pending ops (update patch queue, send local events to event.Event subscribers)
	// - pub applied changes from peers
	// - queueNewHbMsg (hb message type change, push msg to hb send queue)
	propagationInterval = 250 * time.Millisecond

	// subHbRefreshInterval is the minimum interval for update of: sub.hb
	subHbRefreshInterval = 100 * propagationInterval

	countRoutineInterval = 1 * time.Second

	ErrDrained = errors.New("drained command")
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
//		During drain pending op commands (not yet processed by cmdC loop) will receive ErrDrained.
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
func run(ctx context.Context, cmdC <-chan caller, hbRecvQ <-chan *hbtype.Msg, drainDuration time.Duration) {
	d := newData()
	d.log = daemonlogctx.Logger(ctx).With().Str("name", "daemondata").Logger()
	d.log.Info().Msg("starting")
	defer d.log.Info().Msg("stopped")
	d.bus = pubsub.BusFromContext(ctx)

	d.startSubscriptions()
	watchCmd := &durationlog.T{Log: d.log}
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
		d.log.Debug().Msg("draining")
		defer d.log.Debug().Msg("drained")

		tC := time.After(drainDuration)
		for {
			select {
			case <-hbRecvQ:
				// don't hang hbRecvQ writers
			case c := <-cmdC:
				c.SetError(ErrDrained)
			case <-tC:
				d.log.Debug().Msg("drop pending cmds done")
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
			if !needMessage && !gensEqual(d.msgLocalGen, d.pending.Cluster.Node[d.localNode].Status.Gen) {
				needMessage = true
				s := d.pending.Cluster.Node[d.localNode].Status
				if isCtxDone() {
					return
				}
				d.bus.Pub(msgbus.NodeStatusUpdated{Node: d.localNode, Value: *s.DeepCopy()},
					d.labelLocalNode,
				)
			}
			if isCtxDone() {
				return
			}
			d.pubPeerDataChanges()
			select {
			case <-ctx.Done():
				return
			case <-subHbRefreshTicker.C:
				d.setDaemonHb()
				d.log.Debug().Msgf("current hb msg mode %s", d.hbMsgMode[d.localNode])
				needMessage = true
				if subHbRefreshAdaptiveInterval < subHbRefreshInterval {
					subHbRefreshAdaptiveInterval = 2 * subHbRefreshAdaptiveInterval
					subHbRefreshTicker.Reset(subHbRefreshAdaptiveInterval)
					d.log.Debug().Msgf("adapt interval for sub hb stat: %s", subHbRefreshAdaptiveInterval)
				}
			default:
			}
			select {
			case <-ctx.Done():
				return
			case <-countRoutineTicker.C:
				d.pending.Daemon.Routines = runtime.NumGoroutine()
			default:
			}
			if needMessage || d.needMsg {
				hbMsgType := d.hbMessageType
				if isCtxDone() {
					return
				}
				if err := d.queueNewHbMsg(ctx); err != nil {
					d.log.Error().Err(err).Msg("queue hb message")
				} else {
					d.needMsg = false
					if hbMsgType != d.hbMessageType {
						subHbRefreshAdaptiveInterval = propagationInterval
						d.log.Debug().Msgf("hb mg type changed, adapt interval for sub hb stat: %s", subHbRefreshAdaptiveInterval)
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
				d.log.Debug().Msgf("drop rx message message: %s is not cluster member", msg.Nodename)
			}
		case cmd := <-cmdC:
			if c, ok := cmd.(caller); ok {
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
				d.log.Debug().Msgf("%s{...} is not a caller-interface cmd", reflect.TypeOf(cmd))
				d.statCount[idUndef]++
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

func (d *data) startSubscriptions() {
	sub := d.bus.Sub("daemondata")
	sub.AddFilter(msgbus.ClusterConfigUpdated{})
	sub.AddFilter(msgbus.InstanceConfigDeleted{}, d.labelLocalNode)
	sub.AddFilter(msgbus.InstanceConfigUpdated{}, d.labelLocalNode)
	sub.AddFilter(msgbus.InstanceStatusDeleted{}, d.labelLocalNode)
	sub.Start()
	d.sub = sub
}

func (d *data) onSubEvent(i interface{}) {
	switch c := i.(type) {
	case msgbus.ClusterConfigUpdated:
		d.onClusterConfigUpdated(c)
	case msgbus.InstanceConfigDeleted:
		d.onInstanceConfigDeleted(c)
	case msgbus.InstanceConfigUpdated:
		d.onInstanceConfigUpdated(c)
	case msgbus.InstanceStatusDeleted:
		d.onInstanceStatusDeleted(c)
	}
}
