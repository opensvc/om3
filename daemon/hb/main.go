package hb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/clusterhb"
	"github.com/opensvc/om3/v3/core/hbcfg"
	"github.com/opensvc/om3/v3/core/hbtype"
	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/daemondata"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/hb/hbaudit"
	"github.com/opensvc/om3/v3/daemon/hb/hbcrypto"
	"github.com/opensvc/om3/v3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
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

		// retryErr is the error of the last start of the heartbeats
		// whose start failed, by rid, logged when it changes.
		retryErr map[string]string

		// idle are the streams registered to the controller with no
		// stream running, failed or stopped, by id, as "hb#1.tx", by the
		// rid of their heartbeat: listed, to report their state and their
		// error, they are unregistered with their heartbeat section,
		// which leaves no stream running to unregister them.
		idle map[string]idleStream

		// janitorDone is closed when the janitor loop returned: Stop
		// waits for it before it reads the heartbeats running, which the
		// loop starts.
		janitorDone chan struct{}

		// ctx is the main context for the controller, and started hb drivers
		ctx context.Context

		// cancel is the cancel function for msgToTx, msgFromRx, janitor
		cancel context.CancelFunc
		wg     sync.WaitGroup
	}

	// idleStream is a stream registered with no stream running: the rid of
	// its heartbeat, and whether it failed, to be started again, or was
	// stopped, by "om daemon hb stop", to be left so.
	idleStream struct {
		rid    string
		failed bool
	}

	// confer is a heartbeat configured, with the error its driver
	// configured it with, if any.
	confer struct {
		hbcfg.Confer
		err error
	}

	registerTxQueue struct {
		id string
		// msgToSendQueue is the queue on which a tx fetch messages to send
		msgToSendQueue chan []byte
	}
)

func New(_ context.Context, opts ...funcopt.O) *T {
	t := &T{
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/hb").WithPrefix("daemon: hb: main: "),
	}
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Warnf("funcopt apply: %s", err)
	}
	t.txs = make(map[string]hbtype.Transmitter)
	t.rxs = make(map[string]hbtype.Receiver)
	t.readMsgQueue = make(chan *hbtype.Msg)
	t.ridSignature = make(map[string]string)
	t.retryErr = make(map[string]string)
	t.idle = make(map[string]idleStream)
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

	hbaudit.EnableAudit(ctx, "hb.main", t.log, "hb", "hb.main")
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

	// The janitor starts heartbeats: the ones running are read once it
	// returned, or one it started meanwhile would run on.
	if t.janitorDone != nil {
		<-t.janitorDone
	}

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

func (t *T) startHb(hb confer) error {
	var errs error
	if err := t.startHbTx(hb); err != nil {
		errs = errors.Join(errs, fmt.Errorf("start %s.tx failed: %w", hb.Name(), err))
	}
	if err := t.startHbRx(hb); err != nil {
		errs = errors.Join(errs, fmt.Errorf("start %s.rx failed: %w", hb.Name(), err))
	}
	return errs
}

func (t *T) startHbTx(hb confer) error {
	if hb.err != nil {
		t.registerFailed(hb.Name(), hb.Name()+".tx", hb.Type(), hb.err)
		return hb.err
	}
	tx := hb.Tx()
	if tx == nil {
		return fmt.Errorf("nil tx for %s", hb.Name())
	}
	t.ctrlC <- hbctrl.CmdRegister{ID: tx.ID(), Type: hb.Type()}
	delete(t.idle, tx.ID())

	// start debounce msg goroutine to ensure non-blocking write to msgToSendQ:
	// the msgToTxCtx goroutine multiplexes data messages to all hb tx drivers.
	// It can't be stalled because of slow hb transmitter.
	debouncedMsgQ := make(chan []byte)
	msgToSendQ := make(chan []byte)

	if err := tx.Start(t.ctrlC, debouncedMsgQ); err != nil {
		t.setFailed(hb.Name(), tx.ID(), err)
		return err
	}
	go func(ctx context.Context) {
		debounceLatestMsgToTx(ctx, t.msgToTxCtx, msgToSendQ, debouncedMsgQ)
		t.log.Infof("debounceLatestMsgToTx done for %s", tx.ID())
	}(tx.Ctx())

	select {
	case <-t.msgToTxCtx.Done():
		// The daemon stops: the tx is not registered, and nothing else
		// would stop it.
		if err := tx.Stop(); err != nil {
			t.log.Warnf("stop %s, started while stopping: %s", tx.ID(), err)
		}
		return fmt.Errorf("%s: the heartbeats are stopping", tx.ID())
	case t.msgToTxRegister <- registerTxQueue{id: tx.ID(), msgToSendQueue: msgToSendQ}:
		t.txs[hb.Name()] = tx
	}
	return nil
}

func (t *T) startHbRx(hb confer) error {
	if hb.err != nil {
		t.registerFailed(hb.Name(), hb.Name()+".rx", hb.Type(), hb.err)
		return hb.err
	}
	rx := hb.Rx()
	if rx == nil {
		return fmt.Errorf("nil rx for %s", hb.Name())
	}
	t.ctrlC <- hbctrl.CmdRegister{ID: rx.ID(), Type: hb.Type()}
	delete(t.idle, rx.ID())
	if err := rx.Start(t.ctrlC, t.readMsgQueue); err != nil {
		t.setFailed(hb.Name(), rx.ID(), err)
		return err
	}
	t.rxs[hb.Name()] = rx
	return nil
}

// registerFailed lists the stream id of the heartbeat rid failed, with the
// error that keeps it from running as an alert: a heartbeat whose driver
// refused its configuration has no stream to register, and was not listed.
//
// The registration replaces the one of a previous try, and is replaced by
// the one of the stream once it starts.
func (t *T) registerFailed(rid, id, hbType string, err error) {
	t.ctrlC <- hbctrl.CmdRegister{ID: id, Type: hbType}
	t.setFailed(rid, id, err)
}

// setFailed says the stream id of the heartbeat rid registered failed, with
// the error as an alert: the error of a start, as a unicast rx on a port
// another heartbeat listens on, was in the journal only. A start that works
// registers the stream again, with no alert.
func (t *T) setFailed(rid, id string, err error) {
	t.ctrlC <- hbctrl.CmdSetState{ID: id, State: "failed"}
	t.ctrlC <- hbctrl.CmdSetAlert{
		HbID:  id,
		Alert: []daemonsubsystem.Alert{{Severity: "error", Message: err.Error()}},
	}
	t.idle[id] = idleStream{rid: rid, failed: true}
}

// unregisterIdle unregisters the streams of the heartbeat rid listed with no
// stream running, failed or stopped.
func (t *T) unregisterIdle(rid string) {
	for id, stream := range t.idle {
		if stream.rid != rid {
			continue
		}
		t.ctrlC <- hbctrl.CmdUnregister{ID: id}
		delete(t.idle, id)
	}
}

// knownRids returns the heartbeats this manager knows of: the ones recorded
// as started from their configuration, and the ones running, sorted.
func (t *T) knownRids() []string {
	m := make(map[string]bool)
	for rid := range t.ridSignature {
		m[rid] = true
	}
	for rid := range t.txs {
		m[rid] = true
	}
	for rid := range t.rxs {
		m[rid] = true
	}
	for _, stream := range t.idle {
		m[stream.rid] = true
	}
	l := make([]string, 0, len(m))
	for rid := range m {
		l = append(l, rid)
	}
	sort.Strings(l)
	return l
}

// isRunningRid says whether a tx or an rx of the heartbeat runs.
func (t *T) isRunningRid(rid string) bool {
	_, tx := t.txs[rid]
	_, rx := t.rxs[rid]
	return tx || rx
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
		return fmt.Errorf("stop hb rid %s error for %s", rid, strings.Join(failures, ", "))
	}
	t.unregisterIdle(rid)
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

	// A heartbeat whose start failed is not recorded, so a later rescan
	// tries it again, and "om daemon hb restart" can start it meanwhile. The
	// running ones are stopped too when their configuration goes away: only
	// the recorded ones were, and a heartbeat started by a restart after a
	// failed start ran on, its configuration deleted, until the daemon
	// stopped.
	for _, rid := range t.knownRids() {
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
			// Not recorded, as a new heartbeat failing its start:
			// retryFailedHb starts it again.
			delete(t.ridSignature, rid)
			errs = errors.Join(errs, err)
			continue
		}
		t.ridSignature[rid] = newSig
	}
	for rid, newSig := range ridSignatureNew {
		if _, ok := t.ridSignature[rid]; !ok {
			if t.isRunningRid(rid) {
				// Started by a restart after its start failed: it
				// is started again from the configuration, rather
				// than a second time beside itself.
				if err := t.stopHbRid(rid); err != nil {
					errs = errors.Join(errs, err)
					continue
				}
			}
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

// retryFailedHb starts again the heartbeats whose start failed, which the
// rescans only did, on a change of the cluster configuration: a disk signed
// changes none, and its streams stayed failed until restarted by hand on every
// node. A failure is logged when it is not the one of the last try.
//
// A heartbeat started meanwhile by a restart is recorded as it runs, and one
// half started, its tx or its rx alone, is started again whole.
func (t *T) retryFailedHb(ctx context.Context) {
	ridHb, err := t.getHbConfigured(ctx)
	if err != nil {
		t.log.Warnf("retry the failed heartbeats: %s", err)
		return
	}
	for rid, hb := range ridHb {
		if _, ok := t.ridSignature[rid]; ok {
			t.retryFailedStreams(rid, hb)
			continue
		}
		_, tx := t.txs[rid]
		_, rx := t.rxs[rid]
		if tx && rx {
			t.ridSignature[rid] = hb.Signature()
			delete(t.retryErr, rid)
			continue
		}
		if tx || rx {
			if err := t.stopHbRid(rid); err != nil {
				t.log.Warnf("retry %s: %s", rid, err)
				continue
			}
		}
		if err := t.startHb(hb); err != nil {
			if msg := err.Error(); msg != t.retryErr[rid] {
				t.log.Warnf("heartbeat %s start failed, retried every %s: %s", rid, retryFailedInterval, err)
				t.retryErr[rid] = msg
			}
			continue
		}
		t.log.Infof("heartbeat %s started on retry", rid)
		t.ridSignature[rid] = hb.Signature()
		delete(t.retryErr, rid)
	}
}

// retryFailedStreams starts again the streams failed of the heartbeat rid
// recorded as started, as one whose restart by "om daemon hb restart"
// failed: the heartbeat recorded, its retry was skipped, and the stream
// stayed failed until the daemon restarted. A stream stopped by "om daemon
// hb stop" is left stopped.
func (t *T) retryFailedStreams(rid string, hb confer) {
	// The ids are collected first: a start failing again registers its
	// stream anew, which a range over the map could visit again.
	ids := make([]string, 0)
	for id, stream := range t.idle {
		if stream.rid == rid && stream.failed {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		var err error
		if strings.HasSuffix(id, ".tx") {
			err = t.startHbTx(hb)
		} else {
			err = t.startHbRx(hb)
		}
		if err != nil {
			if msg := err.Error(); msg != t.retryErr[id] {
				t.log.Warnf("heartbeat stream %s start failed, retried every %s: %s", id, retryFailedInterval, err)
				t.retryErr[id] = msg
			}
			continue
		}
		t.log.Infof("heartbeat stream %s started on retry", id)
		delete(t.retryErr, id)
	}
	if len(ids) == 0 {
		delete(t.retryErr, rid)
	}
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
					t.log.Errorf("msgToTx: can't unset daemondata HBSendQ: %s", err)
				}
			}()

			// drop pending data from hbSendQ (=> release daemondata queueNewHbMsg())
			tC := time.After(daemonenv.DrainChanDuration)
			for {
				select {
				case <-tC:
					return
				case <-msgC:
					t.log.Tracef("msgToTx: drop msg (done context)")
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
				t.log.Tracef("msgToTx: add %s to hb transmitters", c.id)
				registeredTxMsgQueue[c.id] = c.msgToSendQueue
			case txID := <-t.msgToTxUnregister:
				t.log.Tracef("msgToTx: remove %s from hb transmitters", txID)
				delete(registeredTxMsgQueue, txID)
			case msg := <-msgC:
				b, err := json.Marshal(msg)
				if err != nil {
					err = fmt.Errorf("msgToTx: marshal failure %s for msg %v", err, msg)
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
				t.log.Tracef("msgFromRx drop msg (done context)")
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-statTicker.C:
			t.log.Tracef("msgFromRx received message: %.2f/s, goroutines %d", count/10, runtime.NumGoroutine())
			count = 0
			for peer, updated := range msgTimes {
				if now.Sub(updated) > msgTimeDuration {
					delete(msgTimes, peer)
				}
			}
		case msg := <-t.readMsgQueue:
			peer := msg.Nodename
			last := msgTimes[peer]
			if last.Equal(msg.UpdatedAt) {
				t.log.Tracef("msgFromRx: drop already processed msg %s from %s gens: %v", msg.Kind, msg.Nodename, msg.Gen)
				continue
			}
			if isStaleMsg(msg.UpdatedAt, last) {
				t.log.Debugf("msgFromRx: drop msg %s from %s gens: %v older than the last processed by %s", msg.Kind, msg.Nodename, msg.Gen, last.Sub(msg.UpdatedAt))
				continue
			}
			select {
			case <-ctx.Done():
				// don't hang up when context is done
				return
			case dataMsgRecvQ <- msg:
				t.log.Tracef("msgFromRx: processed msg type %s from %s gens: %v", msg.Kind, msg.Nodename, msg.Gen)
				msgTimes[peer] = msg.UpdatedAt
				count++
			}
		}
	}
}

// staleMsgMaxAge is the age, relative to the last message processed from a
// peer, below which an older message of that peer is dropped as stale.
//
// It covers the heartbeat timeouts: a slow heartbeat, as a disk or a relay
// one, can deliver a message of a peer after a faster one delivered later
// ones, as on its first read after this daemon started, and its receiver no
// longer delivers a message older than its timeout. A message older still is
// taken for one of a peer whose clock was stepped back, and processed, so a
// clock stepped back costs at most this long of dropped messages.
const staleMsgMaxAge = 2 * time.Minute

// retryFailedInterval is the period of the retry of the heartbeats whose start
// failed, as a disk heartbeat on a disk not signed yet.
const retryFailedInterval = 30 * time.Second

// isStaleMsg tells whether a peer message stamped updated is older than the
// last message processed from that peer, stamped last, by less than
// staleMsgMaxAge.
//
// Processing such a message would set back what this node knows of the peer,
// its gens first: the peer would be asked a full message again, and this node
// would stay in full message type, for a state already superseded.
func isStaleMsg(updated, last time.Time) bool {
	if last.IsZero() || !updated.Before(last) {
		return false
	}
	return last.Sub(updated) < staleMsgMaxAge
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

	t.janitorDone = make(chan struct{})
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer close(t.janitorDone)
		started <- true
		sub := pubsub.SubFromContext(ctx, "daemon.hb.janitor")
		sub.AddFilter(&msgbus.ClusterConfigUpdated{}, pubsub.Label{"node", hostname.Hostname()})
		sub.AddFilter(&msgbus.DaemonCtl{})
		sub.Start()
		defer func() {
			if err := sub.Stop(); err != nil {
				t.log.Errorf("subscription stop: %s", err)
			}
		}()
		retryTicker := time.NewTicker(retryFailedInterval)
		defer retryTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-retryTicker.C:
				// The tick and the cancel may be ready at once.
				if ctx.Err() != nil {
					return
				}
				t.retryFailedHb(t.ctx)
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
	var (
		hbI   interface{}
		found bool
		rid   string
	)
	if strings.HasSuffix(hbID, ".rx") {
		rid = strings.TrimSuffix(hbID, ".rx")
		if hbI, found = t.rxs[rid]; !found {
			t.log.Infof("daemonctl %s %s found no %s.rx component", action, hbID, rid)
			return
		}
		defer delete(t.rxs, rid)
	} else if strings.HasSuffix(hbID, ".tx") {
		rid = strings.TrimSuffix(hbID, ".tx")
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
		// listed stopped while its section stays, and unregistered with
		// it: it stayed listed until the daemon restarted.
		id := hbI.(hbtype.IDStopper).ID()
		t.ctrlC <- hbctrl.CmdSetState{ID: id, State: "stopped"}
		t.idle[id] = idleStream{rid: rid}
	}
}

// getHbConfigured returns the heartbeats the configuration defines, by rid,
// each configured. One whose configuration is refused is returned with the
// error: its start lists it failed with it.
func (t *T) getHbConfigured(ctx context.Context) (ridHb map[string]confer, err error) {
	var node *clusterhb.T
	ridHb = make(map[string]confer)
	node, err = clusterhb.New()
	if err != nil {
		return ridHb, err
	}
	for _, h := range node.Hbs() {
		ridHb[h.Name()] = confer{Confer: h, err: h.Configure(ctx)}
	}
	return ridHb, nil
}

func (t *T) getHbConfiguredComponent(ctx context.Context, rid string) (c confer, err error) {
	var node *clusterhb.T
	node, err = clusterhb.New()
	if err != nil {
		t.log.Errorf("clusterhb.NewPath: %s", err)
		return
	}
	for _, h := range node.Hbs() {
		if h.Name() == rid {
			c = confer{Confer: h, err: h.Configure(ctx)}
			return
		}
	}
	err = fmt.Errorf("not found rid")
	return
}

// debounceLatestMsgToTx is used to relay dequeued messages from inQ
// to outC, without blocking on outQ (the last dequeued message from
// inQ will replace the relay bloqued message to outQ).
// the function returns when tx context, or msgToTxCtx is Done
func debounceLatestMsgToTx(txCtx, msgToTxCtx context.Context, inQ <-chan []byte, outC chan<- []byte) {
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
		case <-txCtx.Done():
			return
		case <-msgToTxCtx.Done():
			return
		}
	}
}
