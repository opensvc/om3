package hbdisk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/maps"

	"github.com/opensvc/om3/v3/core/hbtype"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/hb/hbaudit"
	"github.com/opensvc/om3/v3/daemon/hb/hbcrypto"
	"github.com/opensvc/om3/v3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/v3/daemon/hb/hbdedup"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/sign"
)

type (
	// rx holds an hb unicast receiver
	rx struct {
		sync.WaitGroup

		// decodeErrors logs the peers whose messages decrypt and do not
		// decode.
		decodeErrors hbctrl.DecodeErrors
		base         base
		ctx          context.Context
		id           string
		nodes        []string
		timeout      time.Duration
		interval     time.Duration

		// last is the update time of the slot of each peer as last read,
		// written by the clock of that peer. A slot holding the same time
		// again was not written since, whatever the clocks of the two
		// nodes say.
		last map[string]time.Time

		name   string
		log    *plog.Logger
		cmdC   chan<- any
		msgC   chan<- *hbtype.Msg
		cancel func()

		crypto decryptWithNoder

		// dedup holds the frames the other hb links already delivered
		dedup *hbdedup.Cache

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
	ctx, cancel := context.WithCancel(t.ctx)
	t.ctx = ctx
	t.cancel = cancel

	auditName := strings.Replace(t.id, "hb#", "hb:", 1)
	hbaudit.EnableAudit(ctx, t.id, t.log, "hb", auditName, strings.TrimSuffix(auditName, ".rx"))

	t.log.Infof("starting with storage area: metadata_size + (max_slots x slot_size): %d + (%d x %d)", metaSize(t.base.maxSlots), t.base.maxSlots, sign.SlotSize)
	nodeCount := len(t.nodes) + 1
	if t.base.maxSlots < nodeCount {
		cancel()
		return fmt.Errorf("can't start: not enough slots for %d nodes", nodeCount)
	}
	if openErr := t.base.device.open(); openErr != nil {
		if errors.Is(openErr, sign.ErrLegacySignature) {
			t.log.Warnf("device %s: %s", t.base.path, openErr)
		} else {
			err := fmt.Errorf("device %s: %w", t.base.path, openErr)
			t.log.Warnf("startup failed: %s", err)
			cancel()
			return err
		}
	}
	if err := t.base.scanMetadata(append(t.nodes, t.base.localhost)...); err != nil {
		cancel()
		return err
	}

	t.cmdC = cmdC
	t.msgC = msgC

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
		t.dedup = hbdedup.CacheFromContext(ctx)
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
		tickerCheckSignature := time.NewTicker(120 * t.interval)
		defer tickerCheckSignature.Stop()
		for {
			select {
			case <-ticker.C:
				t.crypto = crypto.Load()
				t.onTick()
			case <-tickerCheckSignature.C:
				t.base.checkSignature()
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
	if !t.isNewWrite(nodename, c.Updated) {
		t.log.Tracef("node %s slot %d not written since last read, or too old on first read", nodename, slot)
		return
	}
	key := hbdedup.NewKey(c.Msg)
	if msgNodename, ok := t.dedup.Seen(key); ok {
		// another hb link delivered this very frame: the peer is alive on
		// this one too, but the message is already in the daemon
		if nodename != msgNodename {
			reason := fmt.Sprintf("node %s slot %d was stolen by node %s", nodename, slot, msgNodename)
			t.rescanMetadata(reason)
			return
		}
		t.log.Tracef("node %s slot %d ok, already delivered by another hb", nodename, slot)
		t.cmdC <- hbctrl.CmdSetPeerSuccess{
			Nodename: msgNodename,
			HbID:     t.id,
			Success:  true,
		}
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

	// The message decrypted, from the node owning the slot: the node is
	// alive, whether or not this agent can read what it says.
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: nodename,
		HbID:     t.id,
		Success:  true,
	}
	msg := hbtype.Msg{}
	if err := json.Unmarshal(b, &msg); err != nil {
		t.decodeErrors.Failed(t.log, nodename, err)
		return
	}
	t.decodeErrors.Succeeded(t.log, nodename)
	t.log.Tracef("node %s slot %d ok", nodename, slot)
	t.msgC <- &msg
	t.dedup.Delivered(key, msg.Nodename)
}

// isNewWrite says whether the slot of a peer was written since the last read,
// and records the update time read.
//
// A peer is alive when it wrote its slot since the last read. The age of the
// write, measured against the clock of this node, says nothing once there is
// a last read to compare with: the peer stamps it with its own clock, and a
// peer whose clock runs ahead would look alive for as long as it runs ahead,
// dead or not. The first read has nothing to compare with, so a write too old
// to be the one of a live peer is left for the next one.
func (t *rx) isNewWrite(nodename string, updated time.Time) bool {
	last, seen := t.last[nodename]
	t.last[nodename] = updated
	if seen {
		return !updated.Equal(last)
	}
	return time.Since(updated) <= t.timeout
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
		last:     make(map[string]time.Time),
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
