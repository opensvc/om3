package hbdisk

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
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
	}
)

// ID implements the ID function of Transmitter interface for tx
func (t *tx) ID() string {
	return t.id
}

// Stop implements the Stop function of Transmitter interface for tx
func (t *tx) Stop() error {
	t.log.Debugf("cancelling")
	t.cancel()
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdDelWatcher{
			HbID:     t.id,
			Nodename: node,
		}
	}
	t.Wait()
	t.log.Debugf("wait done")
	return nil
}

// Start implements the Start function of Transmitter interface for tx
func (t *tx) Start(cmdC chan<- interface{}, msgC <-chan []byte) error {
	t.log.Infof("starting")
	if t.base.maxSlots < len(t.nodes) {
		return fmt.Errorf("can't start: not enough slots for %d nodes", len(t.nodes))
	}
	if err := t.base.device.open(); err != nil {
		return err
	}
	if err := t.base.scanMetadata(t.base.localhost); err != nil {
		msg := fmt.Sprintf("initial scan metadata: %s", err)
		t.log.Infof(msg)
		t.alert = append(t.alert, daemonsubsystem.Alert{Severity: "info", Message: msg})
	}
	if slot := t.base.nodeSlot[t.base.localhost]; slot >= 0 {
		t.slot = slot
	}
	reasonTick := fmt.Sprintf("send msg (interval %s)", t.interval)
	ctx, cancel := context.WithCancel(t.ctx)
	t.cancel = cancel
	t.cmdC = cmdC
	t.Add(1)
	go func() {
		defer t.Done()
		t.log.Infof("started")
		defer t.log.Infof("stopped")

		if t.slot < 0 {
			if err := t.allocateSlot(); err != nil {
				t.log.Infof("can't allocate new slot: %s", err)
			}
		}

		t.updateAlertWithSlot()
		t.sendAlert()

		for _, node := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbID:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
			}
		}
		var b []byte
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
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
			}
			if len(b) == 0 {
				continue
			}
			t.log.Debugf(reason)
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

		t.log.Debugf("allocating slot %d for node %s", slot, localhost)
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
	if t.slot < 0 {
		needAllocateReason = "meta data slot is unknown"
	} else if slotInfo, err := t.base.readMetaSlot(t.slot); err != nil {
		needAllocateReason = fmt.Sprintf("can't read meta data slot %d: %s", t.slot, err)
	} else if nodename := nodeFromMetadata(slotInfo); nodename == "" {
		needAllocateReason = fmt.Sprintf("slot %d reset", t.slot)
	} else if nodename != t.base.localhost {
		needAllocateReason = fmt.Sprintf("slot %d stolen by node %s", t.slot, nodename)
	}
	if len(needAllocateReason) > 0 {
		t.log.Infof(needAllocateReason)
		t.alert = make([]daemonsubsystem.Alert, 0)
		defer t.sendAlert()
		if err := t.allocateSlot(); err != nil {
			t.log.Infof("can't allocate new slot: %s", err)
			t.alert = append(t.alert,
				daemonsubsystem.Alert{Severity: "info", Message: needAllocateReason},
				daemonsubsystem.Alert{Severity: "warning", Message: fmt.Sprintf("can't allocate new slot: %s", err)},
			)
			return
		}
		if t.slot < 0 {
			return
		}
		t.updateAlertWithSlot()
	}

	if err := t.base.writeDataSlot(t.slot, b); err != nil { // TODO write timeout?
		t.log.Errorf("write data slot %d: %s", t.slot, err)
		return
	} else {
		t.log.Debugf("written data slot %d len %d", t.slot, len(b))
	}
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
		slot:     -1, // initial slot is unknown
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
