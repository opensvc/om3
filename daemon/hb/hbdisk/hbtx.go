package hbdisk

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/hbtype"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/hb/hbaudit"
	"github.com/opensvc/om3/v3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/sign"
)

type (
	tx struct {
		sync.WaitGroup
		base     base
		ctx      context.Context
		id       string
		nodes    []string
		timeout  time.Duration
		interval time.Duration
		slot     int

		name   string
		log    *plog.Logger
		cmdC   chan<- interface{}
		msgC   chan<- *hbtype.Msg
		cancel func()
		alert  []daemonsubsystem.Alert

		// failing is why the stream can not write its frames, empty
		// while it writes them.
		failing string
	}
)

// ID implements the ID function of Transmitter interface for tx
func (t *tx) ID() string {
	return t.id
}

// Stop implements the Stop function of Transmitter interface for tx
func (t *tx) Stop() error {
	t.log.Tracef("cancelling")
	t.cancel()
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdDelWatcher{
			HbID:     t.id,
			Nodename: node,
		}
	}
	t.Wait()
	t.log.Tracef("wait done")
	t.base.device.close()
	return nil
}

func (t *tx) Ctx() context.Context {
	return t.ctx
}

func (t *tx) streamPeerDesc() string {
	return fmt.Sprintf("→ %s slot %d", t.base.device.file.Name(), t.slot)
}

// Start implements the Start function of Transmitter interface for tx
func (t *tx) Start(cmdC chan<- interface{}, msgC <-chan []byte) error {
	ctx, cancel := context.WithCancel(t.ctx)
	t.ctx = ctx
	t.cancel = cancel
	auditName := strings.Replace(t.id, "hb#", "hb:", 1)
	hbaudit.EnableAudit(ctx, t.id, t.log, "hb", auditName, strings.TrimSuffix(auditName, ".tx"))
	t.log.Infof("starting with storage area: metadata_size + (max_slots x slot_size): %d + (%d x %d)", metaSize(t.base.maxSlots), t.base.maxSlots, sign.SlotSize)
	if t.base.maxSlots < len(t.nodes) {
		cancel()
		return fmt.Errorf("startup failed: not enough slots for %d nodes", len(t.nodes))
	}
	if w := devPathWarning(t.base.path); w != "" {
		t.log.Warnf("%s", w)
	}
	if openErr := t.base.device.open(); openErr != nil {
		if err := t.base.startWith(openErr); err != nil {
			err = fmt.Errorf("device %s: %w", t.base.path, err)
			t.log.Warnf("startup failed: %s", err)
			cancel()
			return err
		}
	}
	if err := t.base.device.checkSize(t.base.maxSlots, len(t.nodes)+1); err != nil {
		t.log.Warnf("startup failed: %s", err)
		t.base.device.close()
		cancel()
		return err
	}
	reasonTick := fmt.Sprintf("send msg (interval %s)", t.interval)
	t.cmdC = cmdC
	t.Add(1)
	go func() {
		defer t.Done()
		t.log.Infof("started")
		defer t.log.Infof("stopped")

		if t.base.invalidSig {
			// A disk not signed gets no slot: the resume claims
			// one once it is signed.
			t.alert = []daemonsubsystem.Alert{t.base.invalidSigAlert()}
			cmdC <- hbctrl.CmdSetState{ID: t.id, State: "failed"}
			t.sendAlert()
		} else if err := t.claimSlot(); err != nil {
			t.fail(fmt.Sprintf("can't allocate a slot: %s", err))
		} else {
			t.updateAlertWithSlot()
			t.sendAlert()
		}

		for _, node := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbID:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
				Desc:     t.streamPeerDesc(),
			}
		}
		var b []byte
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
		tickerCheckSignature := time.NewTicker(signatureCheckPeriod(t.interval, t.base.invalidSig))
		defer tickerCheckSignature.Stop()
		var reason string
		for {
			select {
			case <-ctx.Done():
				return
			case b = <-msgC:
				reason = "send msg"

				// No need to send the next message before a full ticker period.
				ticker.Reset(t.interval)
			case <-ticker.C:
				reason = reasonTick
			case <-tickerCheckSignature.C:
				if t.base.checkSignature() {
					tickerCheckSignature.Reset(signatureCheckPeriod(t.interval, t.base.invalidSig))
					t.onSignatureChange()
				}
				continue
			}
			if len(b) == 0 || t.base.invalidSig {
				// A disk wiped is not written: a slot found
				// reset would be allocated again, on a disk
				// another cluster may be signed for.
				continue
			}
			t.log.Tracef(reason)
			t.send(b)
		}
	}()
	return nil
}

func (t *tx) allocateSlot() error {
	localhost := t.base.localhost
	b := []byte(localhost)
	b = append(b, endOfDataMarker)
	tries := 0
	maxRetryOnConflict := 100
	for i := 0; i < maxRetryOnConflict; i++ {
		tries++
		slot, err := t.base.freeSlot()
		if err != nil {
			return fmt.Errorf("free slot: %w", err)
		}

		t.log.Tracef("allocating slot %d for node %s", slot, localhost)
		if err := t.base.writeMetaSlot(slot, b); err != nil {
			return fmt.Errorf("write mata slot %d: %w", slot, err)
		}
		time.Sleep(time.Duration(250+rand.Intn(250)) * time.Millisecond)

		if b, err := t.base.readMetaSlot(slot); err != nil {
			return fmt.Errorf("read meta slot %d: %w", slot, err)
		} else if peer := nodeFromMetadata(b); peer == localhost {
			t.log.Infof("allocated slot %d", slot)
			t.slot = slot
			return nil
		} else if peer == "" {
			t.log.Infof("slot %d reset", slot)
		} else {
			t.log.Infof("slot %d stolen by node %s", slot, peer)
		}
	}
	return fmt.Errorf("can't allocate slot after %d tries", maxRetryOnConflict)
}

func (t *tx) send(b []byte) {
	var needAllocateReason string
	if t.slot < minimumSlot {
		needAllocateReason = "meta data slot is unknown"
	} else if slotInfo, err := t.base.readMetaSlot(t.slot); err != nil {
		needAllocateReason = fmt.Sprintf("can't read meta data slot %d: %s", t.slot, err)
	} else if nodename := nodeFromMetadata(slotInfo); nodename == "" {
		needAllocateReason = fmt.Sprintf("slot %d reset", t.slot)
	} else if nodename != t.base.localhost {
		needAllocateReason = fmt.Sprintf("slot %d stolen by node %s", t.slot, nodename)
	}
	if len(needAllocateReason) > 0 {
		if t.failing == "" {
			t.log.Infof("%s", needAllocateReason)
		}
		t.alert = make([]daemonsubsystem.Alert, 0)
		if err := t.claimSlot(); err != nil {
			t.fail(fmt.Sprintf("%s, and can't allocate a new one: %s", needAllocateReason, err))
			return
		}
		t.updateAlertWithSlot()
		t.updateDesc()
		if t.failing == "" {
			t.sendAlert()
		}
	}

	if err := t.base.writeDataSlot(t.slot, b); err != nil { // TODO write timeout?
		t.fail(fmt.Sprintf("write data slot %d: %s", t.slot, err))
		return
	} else {
		t.log.Tracef("written data slot %d len %d", t.slot, len(b))
	}
	t.writesAgain()
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdSetPeerSuccess{
			Nodename: node,
			HbID:     t.id,
			Success:  true,
		}
	}
}

func (t *tx) updateAlertWithSlot() {
	t.alert = append(t.alert, getSlotAlert(t.base.localhost, t.slot))
}

// fail reports the stream can not write its frames, for reason. The first
// failure of a reason logs it, reports the stream failed with the reason as
// an error alert, and its peers not beating at once, rather than one timeout
// later: a write failing was logged at each interval, the stream listed
// running and its alerts empty. The next failures of the reason say nothing,
// as v2 logged the first only.
func (t *tx) fail(reason string) {
	if reason == t.failing {
		t.log.Tracef("%s", reason)
		return
	}
	t.failing = reason
	t.log.Errorf("%s", reason)
	t.alert = []daemonsubsystem.Alert{{Severity: "error", Message: reason}}
	t.cmdC <- hbctrl.CmdSetState{ID: t.id, State: "failed"}
	t.base.stopBeating(t.cmdC, t.id, t.nodes)
	t.sendAlert()
}

// writesAgain reports a stream failing writes its frames again.
func (t *tx) writesAgain() {
	if t.failing == "" {
		return
	}
	t.log.Infof("writes again in slot %d, after: %s", t.slot, t.failing)
	t.failing = ""
	t.alert = []daemonsubsystem.Alert{getSlotAlert(t.base.localhost, t.slot)}
	t.cmdC <- hbctrl.CmdSetState{ID: t.id, State: "running"}
	t.updateDesc()
	t.sendAlert()
}

// updateDesc tells the watchers of the stream the slot it writes in, which
// they were added before it was claimed, or before it moved.
func (t *tx) updateDesc() {
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdSetDesc{HbID: t.id, Nodename: node, Desc: t.streamPeerDesc()}
	}
}

// onSignatureChange reports the stream suspended or resumed. Suspended, it
// stops beating at once, rather than one timeout later. Resumed, it claims
// its slot again before it writes: a wipe and a sign keep the metadata, and
// the slot this node holds there.
func (t *tx) onSignatureChange() {
	t.failing = ""
	if t.base.invalidSig {
		t.alert = []daemonsubsystem.Alert{t.base.invalidSigAlert()}
		t.cmdC <- hbctrl.CmdSetState{ID: t.id, State: "failed"}
		t.base.stopBeating(t.cmdC, t.id, t.nodes)
		t.sendAlert()
		return
	}
	t.alert = make([]daemonsubsystem.Alert, 0)
	if err := t.claimSlot(); err != nil {
		t.fail(fmt.Sprintf("can't allocate a slot: %s", err))
		return
	}
	t.updateAlertWithSlot()
	t.updateDesc()
	t.cmdC <- hbctrl.CmdSetState{ID: t.id, State: "running"}
	t.sendAlert()
}

// claimSlot sets the slot this node writes in: the one it holds on the disk,
// else a free one. A slot is never allocated without this look first, which
// would leave the node twice in the metadata, its peers reading the slot
// found first. A disk not signed holds no slot of this node: it gets one
// once signed. It returns the error of an allocation failing.
func (t *tx) claimSlot() error {
	if t.base.invalidSig {
		return nil
	}
	t.slot = 0
	if err := t.base.scanMetadata(t.base.localhost); err != nil {
		msg := fmt.Sprintf("scan metadata: %s", err)
		t.log.Infof("%s", msg)
		t.alert = append(t.alert, daemonsubsystem.Alert{Severity: "info", Message: msg})
	}
	t.base.clearSlotsPastEnd(t.base.localhost)
	if slot := t.base.nodeSlot[t.base.localhost]; slot >= minimumSlot {
		t.slot = slot
		return nil
	}
	return t.allocateSlot()
}

func (t *tx) sendAlert() {
	t.cmdC <- hbctrl.CmdSetAlert{
		HbID:  t.id,
		Alert: append([]daemonsubsystem.Alert{}, t.alert...),
	}
}

func newTx(ctx context.Context, name string, nodes []string, dev string, timeout, interval time.Duration, maxSlots int) *tx {
	id := name + ".tx"
	log := plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbdisk").
		Attr("hb_func", "tx").
		Attr("hb_name", name).
		Attr("hb_id", id).
		WithPrefix("daemon: hb: disk: tx: " + name + ": ")
	return &tx{
		ctx:      ctx,
		id:       id,
		nodes:    nodes,
		timeout:  timeout,
		interval: interval,
		log:      log,
		slot:     0, // initial slot is unknown
		base: base{
			log: log,
			device: device{
				path:     dev,
				metaSize: metaSize(maxSlots),
			},
			maxSlots:  maxSlots,
			localhost: hostname.Hostname(),
		},
	}
}
