/*
Package hbdisk implement a hb disk driver.
A designated disk is sliced into per node data chunks received the exchanged dataset.
*/
package hbdisk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ncw/directio"

	"github.com/opensvc/om3/core/hbcfg"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/plog"
)

type (
	T struct {
		hbcfg.T
	}

	capsule struct {
		Updated time.Time `json:"updated"`
		Msg     []byte    `json:"msg"`
	}

	base struct {
		device
		nodeSlot        map[string]int
		nodeSlotUnknown map[string]bool
		log             *plog.Logger
		localhost       string
		maxSlots        int
	}
)

var (
	// SlotSize is the data size reserved for a single node
	SlotSize = 1024 * 1024

	SlotSizeInt64 = int64(SlotSize)
)

const (
	// PageSize is the directio block size
	PageSize = directio.BlockSize

	// pageSizeInt64 is the int64 conversion of directio block size
	pageSizeInt64 = int64(directio.BlockSize) // Introduce a constant for int64 conversion of PageSize

	endOfDataMarker = '\x00'

	// minimumSlot represents the minimum slot value in the system, used as the
	// base or default starting slot.
	minimumSlot = 1
)

func New() hbcfg.Confer {
	t := &T{}
	var i interface{} = t
	return i.(hbcfg.Confer)
}

func init() {
	hbcfg.Register("disk", New)
}

// Configure implements the Configure function of Confer interface for T
func (t *T) Configure(ctx context.Context) {
	log := plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbdisk").Attr("hb_name", t.Name()).WithPrefix("daemon: hb: disk: " + t.Name() + ": configure:")
	timeout := t.GetDuration("timeout", 9*time.Second)
	interval := t.GetDuration("interval", 4*time.Second)
	if timeout < 2*interval+1*time.Second {
		oldTimeout := timeout
		timeout = interval*2 + 1*time.Second
		log.Warnf("reajust timeout: %s => %s (<interval>*2+1s)", oldTimeout, timeout)
	}

	nodes := t.GetStrings("nodes")
	if len(nodes) == 0 {
		k := key.T{Section: "cluster", Option: "nodes"}
		nodes = t.Config().GetStrings(k)
	}
	dev := t.GetString("dev")
	maxSlots := t.GetInt("max_slots")
	oNodes := hostname.OtherNodes(nodes)
	log.Debugf("timeout=%s interval=%s dev=%s nodes=%s onodes=%s max_slot=%d", timeout, interval, dev, nodes, oNodes, maxSlots)

	t.SetNodes(oNodes)
	t.SetTimeout(timeout)
	signature := fmt.Sprintf("type: hb.disk, disk: %s nodes: %s timeout: %s interval: %s max_slot: %d", dev, nodes, timeout, interval, maxSlots)
	t.SetSignature(signature)
	name := t.Name()
	tx := newTx(ctx, name, oNodes, dev, timeout, interval, maxSlots)
	t.SetTx(tx)
	rx := newRx(ctx, name, oNodes, dev, timeout, interval, maxSlots)
	t.SetRx(rx)
}

func (t *base) scanMetadata(searchedNodes ...string) error {
	var errs error
	var nodeSlotPrevious map[string]int
	if t.nodeSlot != nil {
		nodeSlotPrevious = t.nodeSlot
	} else {
		nodeSlotPrevious = t.nodeSlot
	}
	t.nodeSlot = make(map[string]int)
	t.nodeSlotUnknown = make(map[string]bool)
	defer func(now time.Time) { t.log.Debugf("scanMetadata elapsed %s", time.Since(now)) }(time.Now())
	// Initialize peer configs with all provided searchedNodes and local hostname.
	for _, node := range searchedNodes {
		t.nodeSlot[node] = 0
		t.nodeSlotUnknown[node] = true
	}

	// Process each metadata slot.
	for slot := minimumSlot; slot < t.maxSlots; slot++ {
		b, err := t.device.readMetaSlot(slot)
		if err != nil {
			return fmt.Errorf("read meta slot %d: %w", slot, errors.Join(errs, err))
		}
		index := bytes.IndexRune(b, endOfDataMarker)
		if index < 0 {
			// ignore corrupted meta slot
			continue
		} else if index == 0 {
			// ignore unallocated meta slot, b2.1 abort here
			continue
		}
		nodename := string(b[:index])
		initialSlot, ok := t.nodeSlot[nodename]
		if !ok {
			// ignore non searched node: perhaps a non cluster node, or a cluster node that we are not searching.
			continue
		}
		if initialSlot >= minimumSlot && initialSlot != slot {
			errs = errors.Join(errs, fmt.Errorf("duplicate slot %d for node %s (first %d)", slot, nodename, initialSlot))
			continue
		}
		t.nodeSlot[nodename] = slot
		delete(t.nodeSlotUnknown, nodename)
		if previousSlot, ok := nodeSlotPrevious[nodename]; !ok || previousSlot != slot {
			t.log.Infof("detect slot %d for node %s", slot, nodename)
		}
		if len(t.nodeSlotUnknown) == 0 {
			t.log.Infof("parsed %d slots, found all required slots", slot+1)
			return nil
		}
	}
	return errs
}

// freeSlot scans available slots on the device and returns the first free slot
// index or an error if no free slot is found.
func (t *base) freeSlot() (int, error) {
	for slot := minimumSlot; slot < t.maxSlots; slot++ {
		b, err := t.device.readMetaSlot(slot)
		if err != nil {
			return 0, fmt.Errorf("read meta slot %d: %w", slot, err)
		}
		if len(b) == 0 {
			break
		} else if b[0] != endOfDataMarker {
			continue
		}
		return slot, nil
	}
	return 0, fmt.Errorf("no free slot on dev")
}

func nodeFromMetadata(b []byte) string {
	index := bytes.IndexRune(b, endOfDataMarker)
	if index < 0 {
		return ""
	} else if index == 0 {
		return ""
	}
	return string(b[:index])
}

func metaSize(maxSlots int) int64 {
	return int64(maxSlots * PageSize)
}

func getSlotAlert(nodename string, slot int) daemonsubsystem.Alert {
	msg := fmt.Sprintf("node %s slot %d", nodename, slot)
	level := "info"
	if slot < minimumSlot {
		level = "warning"
	}
	return daemonsubsystem.Alert{Severity: level, Message: msg}
}
