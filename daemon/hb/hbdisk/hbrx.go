package hbdisk

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"golang.org/x/exp/maps"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/daemon/hb/hbcrypto"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/sign"
)

type (
	// rx holds an hb unicast receiver
	rx struct {
		sync.WaitGroup
		base     base
		ctx      context.Context
		id       string
		nodes    []string
		timeout  time.Duration
		interval time.Duration
		last     time.Time

		name   string
		log    *plog.Logger
		cmdC   chan<- any
		msgC   chan<- *hbtype.Msg
		cancel func()

		crypto decryptWithNoder

		// rescanMetadataReason stores the most recent reason for a metadata rescan,
		// helping to prevent excessive logging of the same reason.
		rescanMetadataReason string

		alert []daemonsubsystem.Alert
	}

	decryptWithNoder interface {
		DecryptWithNode(data []byte) ([]byte, string, error)
	}
)

// ID implements the ID function of the Receiver interface for rx
func (t *rx) ID() string {
	return t.id
}

// Stop implements the Stop function of the Receiver interface for rx
func (t *rx) Stop() error {
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
	return nil
}

func (t *rx) streamPeerDesc(node string) string {
	if slot, ok := t.base.nodeSlot[node]; ok {
		return fmt.Sprintf("← %s[%d]", t.base.device.file.Name(), slot)
	} else {
		return fmt.Sprintf("← %s[?]", t.base.device.file.Name())
	}
}

// Start implements the Start function of the Receiver interface for rx
func (t *rx) Start(cmdC chan<- any, msgC chan<- *hbtype.Msg) error {
	t.log.Infof("starting with storage area: metadata_size + (max_slots x slot_size): %d + (%d x %d)", metaSize(t.base.maxSlots), t.base.maxSlots, sign.SlotSize)
	nodeCount := len(t.nodes) + 1
	if t.base.maxSlots < nodeCount {
		return fmt.Errorf("can't start: not enough slots for %d nodes", nodeCount)
	}
	if err := t.base.device.open(); err != nil {
		err := fmt.Errorf("device %s: %w", t.base.path, err)
		t.log.Warnf("startup failed: %s", err)
		return err
	}
	if err := t.base.scanMetadata(append(t.nodes, t.base.localhost)...); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(t.ctx)
	t.cmdC = cmdC
	t.msgC = msgC
	t.cancel = cancel

	for _, node := range t.nodes {
		cmdC <- hbctrl.CmdAddWatcher{
			HbID:     t.id,
			Nodename: node,
			Ctx:      ctx,
			Timeout:  t.timeout,
			Desc:     t.streamPeerDesc(node),
		}
	}

	t.Add(1)
	go func() {
		defer t.Done()
		t.log.Infof("started")
		defer t.log.Infof("stopped")

		t.updateAlertWithSlots()
		t.sendAlert()

		crypto := hbcrypto.CryptoFromContext(ctx)
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.crypto = crypto.Load()
				t.onTick()
			case <-ctx.Done():
				t.cancel()
				return
			}
		}
	}()
	return nil
}

func (t *rx) onTick() {
	if len(t.base.nodeSlotUnknown) > 0 {
		t.rescanMetadata(fmt.Sprintf("missing peers: %s", maps.Keys(t.base.nodeSlotUnknown)))
	}
	for _, node := range t.nodes {
		t.recv(node)
	}
}

func (t *rx) recv(nodename string) {
	slot := t.base.nodeSlot[nodename]
	if slot < minimumSlot {
		return
	}
	c, err := t.base.readDataSlot(slot) // TODO read timeout?
	if err != nil {
		reason := fmt.Sprintf("node %s slot %d: %s", nodename, slot, err)
		t.rescanMetadata(reason)
		return
	}
	if c.Updated.IsZero() {
		t.log.Tracef("node %s slot %d has never been updated", nodename, slot)
		return
	}
	if !t.last.IsZero() && c.Updated == t.last {
		t.log.Tracef("node %s slot %d unchanged since last read", nodename, slot)
		return
	}
	elapsed := time.Now().Sub(c.Updated)
	if elapsed > t.timeout {
		t.log.Tracef("node %s slot %d has not been updated for %s", nodename, slot, elapsed)
		return
	}
	b, msgNodename, err := t.crypto.DecryptWithNode(c.Msg)
	if err != nil {
		t.log.Tracef("node %s slot %d decrypt: %s", nodename, slot, err)
		return
	}

	if nodename != msgNodename {
		reason := fmt.Sprintf("node %s slot %d was stolen by node %s", nodename, slot, msgNodename)
		t.rescanMetadata(reason)
		return
	}

	msg := hbtype.Msg{}
	if err := json.Unmarshal(b, &msg); err != nil {
		t.log.Warnf("node %s slot %d can't unmarshal msg: %s", nodename, slot, err)
		return
	}
	t.log.Tracef("node %s slot %d ok", nodename, slot)
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: msg.Nodename,
		HbID:     t.id,
		Success:  true,
	}
	t.msgC <- &msg
	t.last = c.Updated
}

func (t *rx) rescanMetadata(reason string) {
	t.alert = make([]daemonsubsystem.Alert, 0)
	if reason != t.rescanMetadataReason {
		t.log.Infof("rescan metadata needed: %s", reason)
		t.rescanMetadataReason = reason
	}
	if err := t.base.scanMetadata(append(t.nodes, t.base.localhost)...); err != nil {
		t.log.Infof("rescan metadata: %s", err)
		t.alert = append(t.alert, daemonsubsystem.Alert{Severity: "warning", Message: reason})
	}
	if len(t.base.nodeSlotUnknown) > 0 {
		msg := fmt.Sprintf("nodes without slot: %s", maps.Keys(t.base.nodeSlotUnknown))
		t.alert = append(t.alert, daemonsubsystem.Alert{Severity: "warning", Message: msg})
	}
	t.updateAlertWithSlots()
	t.sendAlert()
}

func (t *rx) updateAlertWithSlots() {
	nodes := make([]string, 0, len(t.base.nodeSlot))
	for nodename := range t.base.nodeSlot {
		nodes = append(nodes, nodename)
	}
	sort.Strings(nodes)
	for _, nodename := range nodes {
		t.alert = append(t.alert, getSlotAlert(nodename, t.base.nodeSlot[nodename]))
	}
}

func (t *rx) sendAlert() {
	t.cmdC <- hbctrl.CmdSetAlert{
		HbID:  t.id,
		Alert: append([]daemonsubsystem.Alert{}, t.alert...),
	}
}

func newRx(ctx context.Context, name string, nodes []string, dev string, timeout, interval time.Duration, maxSlots int) *rx {
	id := name + ".rx"
	log := plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbdisk").
		Attr("hb_func", "rx").
		Attr("hb_name", name).
		Attr("hb_id", id).
		WithPrefix("daemon: hb: disk: rx: " + name + ": ")

	return &rx{
		ctx:      ctx,
		id:       id,
		nodes:    nodes,
		timeout:  timeout,
		interval: interval,
		log:      log,
		base: base{
			log: log,
			device: device{
				path:     dev,
				metaSize: metaSize(maxSlots),
			},
			maxSlots:  maxSlots,
			localhost: hostname.Hostname(),
		},
		alert: make([]daemonsubsystem.Alert, 0),
	}
}
