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
		peerConfigs
		device
		log *plog.Logger
	}

	peerConfigs map[string]peerConfig

	peerConfig struct {
		Slot int
	}
)

var (
	// MetaSize is the size of the header reserved on dev to store the
	// slot allocations.
	// A 4MB meta size can index 1024 nodes if pagesize is 4k.
	MetaSize      = 4 * 1024 * 1024
	MetaSizeInt64 = int64(MetaSize)

	// SlotSize is the data size reserved for a single node
	SlotSize = 1024 * 1024

	SlotSizeInt64 = int64(SlotSize)

	// MaxSlots is maximum number of slots that can fit in MetaSize
	MaxSlots = MetaSize / PageSize
)

const (
	// PageSize is the directio block size
	PageSize = directio.BlockSize

	// pageSizeInt64 is the int64 conversion of directio block size
	pageSizeInt64 = int64(directio.BlockSize) // Introduce a constant for int64 conversion of PageSize

	endOfDataMarker = '\x00'
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
	oNodes := hostname.OtherNodes(nodes)
	log.Debugf("timeout=%s interval=%s dev=%s nodes=%s onodes=%s", timeout, interval, dev, nodes, oNodes)
	log.Infof("storage area is metadata_size + (max_slots x slot_size): %d + (%d x %d)", MetaSize, MaxSlots, SlotSize)

	t.SetNodes(oNodes)
	t.SetTimeout(timeout)
	signature := fmt.Sprintf("type: hb.disk, disk: %s nodes: %s timeout: %s interval: %s", dev, nodes, timeout, interval)
	t.SetSignature(signature)
	name := t.Name()
	tx := newTx(ctx, name, oNodes, dev, timeout, interval)
	t.SetTx(tx)
	rx := newRx(ctx, name, oNodes, dev, timeout, interval)
	t.SetRx(rx)
}

func (t *base) loadPeerConfig(nodes []string) error {
	var errs error
	t.peerConfigs = make(peerConfigs)

	// Initialize peer configs with all provided nodes and local hostname.
	for _, node := range append(nodes, hostname.Hostname()) {
		t.peerConfigs[node] = newPeerConfig()
	}

	// Process each metadata slot.
	for slot := 0; slot < MaxSlots; slot++ {
		b, err := t.device.readMetaSlot(slot)
		if err != nil {
			return fmt.Errorf("read meta slot %d: %w", slot, errors.Join(errs, err))
		}
		nodename := string(b[:bytes.IndexRune(b, '\x00')])
		data, ok := t.peerConfigs[nodename]
		if !ok {
			// foreign node
			t.log.Debugf("skip foreign node detected in metadata slot %d: %s", slot, nodename)
			continue
		}
		if data.Slot > 0 && data.Slot != slot {
			errs = errors.Join(errs, fmt.Errorf("duplicate slot %d for node %s (first %d)", slot, nodename, data.Slot))
			continue
		}
		t.log.Infof("detect slot %d for node %s", slot, nodename)
	}
	return errs
}

func (t *base) allocateSlot(nodename string) (peerConfig, error) {
	conf := newPeerConfig()
	conf.Slot = t.peerConfigs.freeSlot()
	if conf.Slot < 0 {
		return conf, errors.New("no free slot on dev")
	}
	b := []byte(nodename)
	b = append(b, endOfDataMarker)
	if err := t.writeMetaSlot(conf.Slot, b); err != nil {
		return conf, fmt.Errorf("write mata slot %d: %w", conf.Slot, err)
	}
	t.peerConfigs[nodename] = conf
	return conf, nil
}

func (t *base) getPeer(s string) (peerConfig, error) {
	data := t.peerConfigs.Get(s)
	if data.Slot >= 0 {
		return data, nil
	}
	return t.allocateSlot(s)
}

func (t peerConfigs) Get(s string) peerConfig {
	if data, ok := t[s]; ok {
		return data
	} else {
		return newPeerConfig()
	}
}

func (t peerConfigs) usedSlots() map[int]any {
	m := make(map[int]any)
	for _, data := range t {
		m[data.Slot] = nil
	}
	return m
}

func (t peerConfigs) freeSlot() int {
	used := t.usedSlots()
	for slot := 0; slot < MaxSlots; slot++ {
		if _, ok := used[slot]; !ok {
			return slot
		}
	}
	return -1
}

func newPeerConfig() peerConfig {
	return peerConfig{
		Slot: -1,
	}
}
