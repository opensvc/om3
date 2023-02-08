package daemondata

import (
	"context"
	"reflect"
	"runtime"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/callcount"
	"github.com/opensvc/om3/util/durationlog"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/jsondelta"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	caller interface {
		call(context.Context, *data)
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
		counterCmd    chan<- interface{}
		log           zerolog.Logger
		bus           *pubsub.Bus

		// msgLocalGen hold the latest published msg gen for localhost
		msgLocalGen map[string]uint64
		hbSendQ     chan<- hbtype.Msg

		// subHbMode holds the hb mode of cluster nodes:
		//
		// for local node: value is set during func (d *data) getHbMessage()
		// for peer:  value it set during func (d *data) onReceiveHbMsg
		// It has same value as subHbMsgType, except when type is patch where it represents size of patch queue
		subHbMode map[string]string

		// subHbMsgType track the hb message type of cluster nodes
		// - localhost associated value is changed during setNextMsgType
		// - other nodes associated value is changed during onReceiveHbMsg
		subHbMsgType map[string]string

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

	labelLocalNode = pubsub.Label{"node", hostname.Hostname()}
)

func PropagationInterval() time.Duration {
	return propagationInterval
}

// run the daemondata loop
//
// the loop does following action in order
//
//	 1- on propagate ticker:
//	   commitPendingOps
//	   pubPeerDataChanges
//	   update sub hb stat on adaptive ticker (from 250ms to 25s)
//	   queueNewHbMsg when hb message is needed
//
//	 2- read hbrx message from queue -> onReceiveHbMsg
//	    apply ping
//	    or apply full
//	    or apply patch
//
//	3- process daemondata cmd
func run(ctx context.Context, cmdC <-chan interface{}, hbRecvQ <-chan *hbtype.Msg) {
	counterCmd, cancel := callcount.Start(ctx, idToName)
	defer cancel()
	d := newData(counterCmd)
	d.log = daemonlogctx.Logger(ctx).With().Str("name", "daemondata").Logger()
	d.log.Info().Msg("starting")
	defer d.log.Info().Msg("stopped")
	d.bus = pubsub.BusFromContext(ctx)

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

	d.hbSendQ = daemonctx.HBSendQ(ctx)

	for {
		select {
		case <-ctx.Done():
			d.log.Debug().Msg("drop pending cmds")
			tC := time.After(100 * time.Millisecond)
			for {
				select {
				case <-hbRecvQ:
					// don't hang hbRecvQ writers
				case c := <-cmdC:
					dropCmd(ctx, c)
				case <-tC:
					d.log.Debug().Msg("drop pending cmds done")
					return
				}
			}
		case <-propagationTicker.C:
			needMessage := d.commitPendingOps()
			if !needMessage && !gensEqual(d.msgLocalGen, d.pending.Cluster.Node[d.localNode].Status.Gen) {
				needMessage = true
				s := d.pending.Cluster.Node[d.localNode].Status
				d.bus.Pub(
					msgbus.NodeStatusUpdated{
						Node:  d.localNode,
						Value: *s.DeepCopy(),
					},
					labelLocalNode,
				)
			}
			d.pubPeerDataChanges()
			select {
			case <-subHbRefreshTicker.C:
				d.setSubHb()
				d.log.Debug().Msgf("current hb msg mode %s", d.subHbMode[d.localNode])
				needMessage = true
				if subHbRefreshAdaptiveInterval < subHbRefreshInterval {
					subHbRefreshAdaptiveInterval = 2 * subHbRefreshAdaptiveInterval
					subHbRefreshTicker.Reset(subHbRefreshAdaptiveInterval)
					d.log.Debug().Msgf("adapt interval for sub hb stat: %s", subHbRefreshAdaptiveInterval)
				}
			default:
			}
			select {
			case <-countRoutineTicker.C:
				d.pending.Monitor.Routines = runtime.NumGoroutine()
			default:
			}
			if needMessage || d.needMsg {
				hbMsgType := d.hbMessageType
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
			d.onReceiveHbMsg(msg)
		case cmd := <-cmdC:
			if c, ok := cmd.(caller); ok {
				beginCmd <- cmd
				c.call(ctx, d)
				endCmd <- true
			} else {
				d.log.Debug().Msgf("%s{...} is not a caller-interface cmd", reflect.TypeOf(cmd))
				counterCmd <- idUndef
			}
		}
	}
}

type (
	errorSetter interface {
		setError(context.Context, error)
	}

	doneSetter interface {
		setDone(context.Context, bool)
	}
)

// dropCmd drops commands with side effects
func dropCmd(ctx context.Context, c interface{}) {
	// TODO implement all side effects
	switch cmd := c.(type) {
	case errorSetter:
		cmd.setError(ctx, nil)
	case doneSetter:
		cmd.setDone(ctx, true)
	}
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
