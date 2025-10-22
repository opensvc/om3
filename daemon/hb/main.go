package hb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/core/clusterhb"
	"github.com/opensvc/om3/core/hbcfg"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/hb/hbcrypto"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	T struct {
		log *plog.Logger
		txs map[string]hbtype.Transmitter
		rxs map[string]hbtype.Receiver

		ctrl  *hbctrl.C
		ctrlC chan<- any

		readMsgQueue chan *hbtype.Msg

		msgToTxRegister   chan registerTxQueue
		msgToTxUnregister chan string
		msgToTxCtx        context.Context

		ridSignature map[string]string

		// ctx is the main context for the controller, and started hb drivers
		ctx context.Context

		// cancel is the cancel function for msgToTx, msgFromRx, janitor
		cancel context.CancelFunc
		wg     sync.WaitGroup
	}

	registerTxQueue struct {
		id string
		// msgToSendQueue is the queue on which a tx fetch messages to send
		msgToSendQueue chan []byte
	}
)

func New(_ context.Context, opts ...funcopt.O) *T {
	t := &T{
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/hb").WithPrefix("daemon: hb: "),
	}
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Warnf("funcopt apply: %s", err)
	}
	t.txs = make(map[string]hbtype.Transmitter)
	t.rxs = make(map[string]hbtype.Receiver)
	t.readMsgQueue = make(chan *hbtype.Msg)
	t.ridSignature = make(map[string]string)
	return t
}

// Start starts the heartbeat components
//
// It starts:
// with ctx:
//   - the hb controller to maintain heartbeat status and peers
//     It is firstly started and lastly stopped
//   - hb drivers
//
// with cancelable context
// - the dispatcher of messages to send to hb tx components
// - the dispatcher of read messages from hb rx components to daemon data
// - the goroutine responsible for hb drivers lifecycle
func (t *T) Start(ctx context.Context) error {
	t.log.Infof("starting")

	// we have to start controller routine first (it will be used by hb drivers)
	// It uses main context ctx: it is the last go routine to stop (after t.cancel() & t.wg.Wait())
	t.ctrl = hbctrl.New()
	t.ctrlC = t.ctrl.Start(ctx)

	// t.ctx will be used to start hb drivers
	t.ctx = ctx

	// create cancelable context to cancel other routines
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	err := t.msgToTx(ctx)
	if err != nil {
		return err
	}

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.msgFromRx(ctx)
	}()

	t.janitor(ctx)
	t.log.Infof("started")
	return nil
}

func (t *T) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")

	// this will cancel janitor, msgToTx, msgFromRx and hb drivers context
	t.cancel()

	hbToStop := make([]hbtype.IDStopper, 0)
	var failedIDs []string
	for _, hb := range t.txs {
		hbToStop = append(hbToStop, hb)
	}
	for _, hb := range t.rxs {
		hbToStop = append(hbToStop, hb)
	}
	for _, hb := range hbToStop {
		if err := t.stopHb(hb); err != nil {
			t.log.Errorf("stop %s: %s", hb.ID(), err)
			failedIDs = append(failedIDs, hb.ID())
		}
	}
	if len(failedIDs) > 0 {
		return fmt.Errorf("failure while stopping heartbeat %s", strings.Join(failedIDs, ", "))
	}

	t.wg.Wait()

	// We can now stop the controller
	if err := t.ctrl.Stop(); err != nil {
		t.log.Errorf("stop hbctrl: %s", err)
	}

	return nil
}

func (t *T) stopHb(hb hbtype.IDStopper) error {
	hbID := hb.ID()
	switch hb.(type) {
	case hbtype.Transmitter:
		select {
		case <-t.msgToTxCtx.Done():
			// don't hang up when context is done
		case t.msgToTxUnregister <- hbID:
		}
	}
	t.ctrlC <- hbctrl.CmdUnregister{ID: hbID}
	return hb.Stop()
}

func (t *T) startHb(hb hbcfg.Confer) error {
	var errs error
	if err := t.startHbTx(hb); err != nil {
		errs = errors.Join(errs, fmt.Errorf("start %s.tx failed: %w", hb.Name(), err))
	}
	if err := t.startHbRx(hb); err != nil {
		errs = errors.Join(errs, fmt.Errorf("start %s.rx failed: %w", hb.Name(), err))
	}
	return errs
}

func (t *T) startHbTx(hb hbcfg.Confer) error {
	tx := hb.Tx()
	if tx == nil {
		return fmt.Errorf("nil tx for %s", hb.Name())
	}
	t.ctrlC <- hbctrl.CmdRegister{ID: tx.ID(), Type: hb.Type()}

	// start debounce msg goroutine to ensure non-blocking write to msgToSendQ:
	// the msgToTxCtx goroutine multiplexes data messages to all hb tx drivers.
	// It can't be stalled because of slow hb transmitter.
	debouncedMsgQ := make(chan []byte)
	msgToSendQ := make(chan []byte)
	go debounceLatestMsgToTx(t.msgToTxCtx, msgToSendQ, debouncedMsgQ)

	if err := tx.Start(t.ctrlC, debouncedMsgQ); err != nil {
		t.ctrlC <- hbctrl.CmdSetState{ID: tx.ID(), State: "failed"}
		return err
	}
	select {
	case <-t.msgToTxCtx.Done():
		// don't hang up when context is done
	case t.msgToTxRegister <- registerTxQueue{id: tx.ID(), msgToSendQueue: msgToSendQ}:
		t.txs[hb.Name()] = tx
	}
	return nil
}

func (t *T) startHbRx(hb hbcfg.Confer) error {
	rx := hb.Rx()
	if rx == nil {
		return fmt.Errorf("nil rx for %s", hb.Name())
	}
	t.ctrlC <- hbctrl.CmdRegister{ID: rx.ID(), Type: hb.Type()}
	if err := rx.Start(t.ctrlC, t.readMsgQueue); err != nil {
		t.ctrlC <- hbctrl.CmdSetState{ID: rx.ID(), State: "failed"}
		return err
	}
	t.rxs[hb.Name()] = rx
	return nil
}

func (t *T) stopHbRid(rid string) error {
	errCount := 0
	failures := make([]string, 0)
	if tx, ok := t.txs[rid]; ok {
		if err := t.stopHb(tx); err != nil {
			failures = append(failures, "tx")
			errCount++
		} else {
			delete(t.txs, rid)
		}
	}
	if rx, ok := t.rxs[rid]; ok {
		if err := t.stopHb(rx); err != nil {
			failures = append(failures, "rx")
			errCount++
		} else {
			delete(t.rxs, rid)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("stop hb rid %s error for " + strings.Join(failures, ", "))
	}
	return nil
}

// rescanHb updates the running heartbeats from existing configuration
//
// To avoid hold resources, the updates are done in this order:
// 1- stop the running heartbeats that don't anymore exist in configuration
// 2- stop the running heartbeats where configuration has been changed
// 3- start the configuration changed stopped heartbeats
// 4- start the new configuration heartbeats
func (t *T) rescanHb(ctx context.Context) error {
	var errs error
	ridHb, err := t.getHbConfigured(ctx)
	if err != nil {
		return err
	}
	ridSignatureNew := make(map[string]string)
	for rid, hb := range ridHb {
		ridSignatureNew[rid] = hb.Signature()
	}

	for rid := range t.ridSignature {
		if _, ok := ridSignatureNew[rid]; ok {
			continue
		}
		t.log.Infof("heartbeat config deleted %s => stopping", rid)
		if err := t.stopHbRid(rid); err == nil {
			delete(t.ridSignature, rid)
		} else {
			errs = errors.Join(errs, err)
		}
	}
	// Stop first to release connection holders
	stoppedRids := make(map[string]string)
	for rid, newSig := range ridSignatureNew {
		if sig, ok := t.ridSignature[rid]; ok {
			if sig != newSig {
				t.log.Infof("heartbeat config changed %s => stopping", rid)
				if err := t.stopHbRid(rid); err != nil {
					errs = errors.Join(errs, err)
					continue
				}
				stoppedRids[rid] = newSig
			}
		}
	}
	for rid, newSig := range stoppedRids {
		t.log.Infof("heartbeat config changed %s => starting (from stopped)", rid)
		if err := t.startHb(ridHb[rid]); err != nil {
			errs = errors.Join(errs, err)
		}
		t.ridSignature[rid] = newSig
	}
	for rid, newSig := range ridSignatureNew {
		if _, ok := t.ridSignature[rid]; !ok {
			t.log.Infof("heartbeat config new %s => starting", rid)
			if err := t.startHb(ridHb[rid]); err != nil {
				errs = errors.Join(errs, err)
				continue
			}
		}
		t.ridSignature[rid] = newSig
	}
	return errs
}

// msgToTx starts the goroutine to multiplex data messages to hb tx drivers
//
// It ends when ctx is done
func (t *T) msgToTx(ctx context.Context) error {
	msgC := make(chan hbtype.Msg)
	databus := daemondata.FromContext(ctx)
	if err := databus.SetHBSendQ(msgC); err != nil {
		return fmt.Errorf("msgToTx can't set daemondata HBSendQ")
	}
	t.msgToTxRegister = make(chan registerTxQueue)
	t.msgToTxUnregister = make(chan string)
	t.msgToTxCtx = ctx
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer t.log.Infof("multiplexer message to hb tx drivers stopped")
		t.log.Infof("multiplexer message to hb tx drivers started")
		registeredTxMsgQueue := make(map[string]chan []byte)
		defer func() {
			// We have to async ask daemondata to not anymore write to hbSendQ
			// async because daemon data can be waiting on running queueNewHbMsg():
			//    hbSendQ <- msg
			go func() {
				if err := databus.SetHBSendQ(nil); err != nil {
					t.log.Errorf("msgToTx can't unset daemondata HBSendQ: %s", err)
				}
			}()

			// drop pending data from hbSendQ (=> release daemondata queueNewHbMsg())
			tC := time.After(daemonenv.DrainChanDuration)
			for {
				select {
				case <-tC:
					return
				case <-msgC:
					t.log.Debugf("msgToTx drop msg (done context)")
				case <-t.msgToTxRegister:
				case <-t.msgToTxUnregister:
				}
			}
		}()

		crypto := hbcrypto.CryptoFromContext(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case c := <-t.msgToTxRegister:
				t.log.Debugf("add %s to hb transmitters", c.id)
				registeredTxMsgQueue[c.id] = c.msgToSendQueue
			case txID := <-t.msgToTxUnregister:
				t.log.Debugf("remove %s from hb transmitters", txID)
				delete(registeredTxMsgQueue, txID)
			case msg := <-msgC:
				b, err := json.Marshal(msg)
				if err != nil {
					err = fmt.Errorf("marshal failure %s for msg %v", err, msg)
					continue
				}
				cipher := crypto.Load()
				if cipher == nil {
					continue
				}
				b, err = cipher.Encrypt(b)
				if err != nil {
					continue
				}
				for _, txQueue := range registeredTxMsgQueue {
					select {
					case <-ctx.Done():
						// don't hang up when context is done
						return
					case txQueue <- b:
					}
				}
			}
		}
	}()
	return nil
}

// msgFromRx get hbrx decoded messages from readMsgQueue, and
// forward the decoded hb message to daemondata HBRecvMsgQ.
//
// When multiple hb rx are running, we can get multiple times the same hb message,
// but only one hb decoded message is forwarded to daemondata HBRecvMsgQ
//
// It ends when ctx is done
func (t *T) msgFromRx(ctx context.Context) {
	defer t.log.Infof("message receiver from hb rx drivers stopped")
	t.log.Infof("message receiver from hb rx drivers started")
	count := 0.0
	statTicker := time.NewTicker(60 * time.Second)
	defer statTicker.Stop()
	dataMsgRecvQ := daemonctx.HBRecvMsgQ(ctx)
	msgTimes := make(map[string]time.Time)
	msgTimeDuration := 10 * time.Minute
	defer func() {
		tC := time.After(daemonenv.DrainChanDuration)
		for {
			select {
			case <-tC:
				return
			case <-t.readMsgQueue:
				t.log.Debugf("msgFromRx drop msg (done context)")
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-statTicker.C:
			t.log.Debugf("received message: %.2f/s, goroutines %d", count/10, runtime.NumGoroutine())
			count = 0
			for peer, updated := range msgTimes {
				if now.Sub(updated) > msgTimeDuration {
					delete(msgTimes, peer)
				}
			}
		case msg := <-t.readMsgQueue:
			peer := msg.Nodename
			if msgTimes[peer].Equal(msg.UpdatedAt) {
				t.log.Debugf("drop already processed msg %s from %s gens: %v", msg.Kind, msg.Nodename, msg.Gen)
				continue
			}
			select {
			case <-ctx.Done():
				// don't hang up when context is done
				return
			case dataMsgRecvQ <- msg:
				t.log.Debugf("processed msg type %s from %s gens: %v", msg.Kind, msg.Nodename, msg.Gen)
				msgTimes[peer] = msg.UpdatedAt
				count++
			}
		}
	}
}

// janitor starts the goroutine responsible for hb drivers lifecycle.
//
// It ends when ctx is done.
//
// It watches cluster InstanceConfigUpdated and DaemonCtl to (re)start hb drivers
// When a hb driver is started, it will use the main context t.ctx.
func (t *T) janitor(ctx context.Context) {
	started := make(chan bool)

	if err := t.rescanHb(ctx); err != nil {
		t.log.Errorf("initial rescan on janitor hb start: %s", err)
	}

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		started <- true
		sub := pubsub.SubFromContext(ctx, "daemon.hb")
		sub.AddFilter(&msgbus.ClusterConfigUpdated{}, pubsub.Label{"node", hostname.Hostname()})
		sub.AddFilter(&msgbus.DaemonCtl{})
		sub.Start()
		defer func() {
			if err := sub.Stop(); err != nil {
				t.log.Errorf("subscription stop: %s", err)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case i := <-sub.C:
				switch msg := i.(type) {
				case *msgbus.DaemonCtl:
					hbID := msg.Component
					action := msg.Action
					if !strings.HasPrefix(hbID, "hb#") {
						continue
					}
					t.log.Infof("handle event DaemonCtl with action %s on component %s", action, hbID)
					switch msg.Action {
					case "stop":
						t.log.Infof("stopping %s", hbID)
						t.daemonCtlStop(hbID, action)
					case "start":
						if hbI := strings.TrimSuffix(hbID, ".tx"); hbI != hbID {
							if _, found := t.txs[hbI]; found {
								t.log.Infof("start %s skipped: already running", hbID)
								continue
							}
						} else if hbI := strings.TrimSuffix(hbID, ".rx"); hbI != hbID {
							if _, found := t.rxs[hbI]; found {
								t.log.Infof("start %s skipped: already running", hbID)
								continue
							}
						} else {
							t.log.Infof("start %s skipped: not a tx/rx pair", hbID)
							continue
						}
						t.log.Infof("starting %s", hbID)
						t.daemonCtlStart(t.ctx, hbID, action)
					case "restart":
						t.log.Infof("restart %s: stopping", hbID)
						t.daemonCtlStop(hbID, action)
						t.log.Infof("restart %s:starting", hbID)
						t.daemonCtlStart(t.ctx, hbID, action)
					}
				case *msgbus.ClusterConfigUpdated:
					t.log.Infof("rescan heartbeat configurations (local cluster config changed)")
					if err := t.rescanHb(t.ctx); err != nil {
						t.log.Errorf("rescan heartbeat configurations has errors: %s", err)
					} else {
						t.log.Infof("rescan heartbeat configurations done")
					}
				}
			}
		}
	}()
	<-started
}

func (t *T) daemonCtlStart(ctx context.Context, hbID string, action string) {
	var rid string
	if strings.HasSuffix(hbID, ".rx") {
		rid = strings.TrimSuffix(hbID, ".rx")
	} else if strings.HasSuffix(hbID, ".tx") {
		rid = strings.TrimSuffix(hbID, ".tx")
	} else {
		t.log.Infof("daemonctl %s found no component for %s", action, hbID)
		return
	}
	h, err := t.getHbConfiguredComponent(ctx, rid)
	if err != nil {
		t.log.Infof("daemonctl %s found no component for %s (rid: %s): %s", action, hbID, rid, err)
		return
	}
	if strings.HasSuffix(hbID, ".rx") {
		if err := t.startHbRx(h); err != nil {
			t.log.Errorf("daemonctl %s %s start rx failed: %s", action, hbID, err)
			return
		}
	} else {
		if err := t.startHbTx(h); err != nil {
			t.log.Errorf("daemonctl %s %s start tx failed: %s", action, hbID, err)
			return
		}
	}
}

func (t *T) daemonCtlStop(hbID string, action string) {
	var hbI interface{}
	var found bool
	if strings.HasSuffix(hbID, ".rx") {
		rid := strings.TrimSuffix(hbID, ".rx")
		if hbI, found = t.rxs[rid]; !found {
			t.log.Infof("daemonctl %s %s found no %s.rx component", action, hbID, rid)
			return
		}
		defer delete(t.rxs, rid)
	} else if strings.HasSuffix(hbID, ".tx") {
		rid := strings.TrimSuffix(hbID, ".tx")
		if hbI, found = t.txs[rid]; !found {
			t.log.Infof("daemonctl %s %s found no %s.tx component", action, hbID, rid)
			return
		}
		defer delete(t.txs, rid)
	} else {
		t.log.Infof("daemonctl %s %s found no component", action, hbID)
		return
	}
	// delete(t.txs, rid)
	t.log.Infof("ask to %s %s", action, hbID)
	switch hbI.(type) {
	case hbtype.Transmitter:
		select {
		case <-t.msgToTxCtx.Done():
		// don't hang up when context is done
		case t.msgToTxUnregister <- hbID:
		}
	}
	if err := hbI.(hbtype.IDStopper).Stop(); err != nil {
		t.log.Errorf("daemonctl %s %s stop failed: %s", action, hbID, err)
	} else {
		t.ctrlC <- hbctrl.CmdSetState{ID: hbI.(hbtype.IDStopper).ID(), State: "stopped"}
	}
}

func (t *T) getHbConfigured(ctx context.Context) (ridHb map[string]hbcfg.Confer, err error) {
	var node *clusterhb.T
	ridHb = make(map[string]hbcfg.Confer)
	node, err = clusterhb.New()
	if err != nil {
		return ridHb, err
	}
	for _, h := range node.Hbs() {
		h.Configure(ctx)
		ridHb[h.Name()] = h
	}
	return ridHb, nil
}

func (t *T) getHbConfiguredComponent(ctx context.Context, rid string) (c hbcfg.Confer, err error) {
	var node *clusterhb.T
	node, err = clusterhb.New()
	if err != nil {
		t.log.Errorf("clusterhb.NewPath: %s", err)
		return
	}
	for _, h := range node.Hbs() {
		h.Configure(ctx)
		if h.Name() == rid {
			c = h
			return
		}
	}
	err = fmt.Errorf("not found rid")
	return
}

// debounceLatestMsgToTx is used to relay dequeued messages from inQ
// to outC, without blocking on outQ (the last dequeued message from
// inQ will replace the relay bloqued message to outQ).
func debounceLatestMsgToTx(ctx context.Context, inQ <-chan []byte, outC chan<- []byte) {
	var (
		b []byte
		o chan<- []byte
	)
	for {
		select {
		case b = <-inQ:
			o = outC
		case o <- b:
			o = nil
		case <-ctx.Done():
			return
		}
	}
}
