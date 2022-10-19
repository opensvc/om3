/*
Package hbdisk implement a hb disk driver.
A designated disk is sliced into per node data chunks received the exchanged dataset.
*/
package hbdisk

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/ncw/directio"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/core/hbcfg"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/xerrors"
)

type (
	T struct {
		hbcfg.T
	}

	base struct {
		peerConfigs
		device
		log zerolog.Logger
	}
	peerConfigs map[string]peerConfig
	peerConfig  struct {
		Slot int
	}
	device struct {
		file *os.File
	}
)

var (
	// PageSize is the directio block size
	PageSize = directio.BlockSize

	// MetaSize is the size of the header reserved on dev to store the
	// slot allocations.
	// A 4MB meta size can index 1024 nodes if pagesize is 4k.
	MetaSize = 4 * 1024 * 1024

	// SlotSize is the data size reserved for a single node
	SlotSize = 1024 * 1024

	// MaxSlot is maximum number of slots that can fit in MetaSize
	MaxSlots = MetaSize / PageSize
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
	log := daemonlogctx.Logger(ctx).With().Str("type", t.Name()).Logger()
	timeout := t.GetDuration("timeout", 9*time.Second)
	interval := t.GetDuration("interval", 4*time.Second)
	if timeout < 2*interval+1*time.Second {
		oldTimeout := timeout
		timeout = interval*2 + 1*time.Second
		log.Warn().Msgf("reajust timeout: %s => %s (<interval>*2+1s)", oldTimeout, timeout)
	}
	dev := t.GetString("dev")
	if dev == "" {
		log.Error().Msgf("no %s.dev is not set in node.conf", t.Name())
		return
	}
	newDev, err := filepath.EvalSymlinks(dev)
	if err != nil {
		log.Error().Err(err).Msgf("%s eval symlink", dev)
		return
	}

	isBlockDevice, err := file.IsBlockDevice(newDev)
	if os.IsNotExist(err) {
		log.Error().Msgf("%s does not exist", dev)
		return
	} else if err != nil {
		log.Error().Err(err).Msgf("configure")
	}

	isCharDevice, err := file.IsCharDevice(newDev)
	if os.IsNotExist(err) {
		log.Error().Msgf("%s does not exist", dev)
		return
	} else if err != nil {
		log.Error().Err(err).Msgf("configure")
	}

	var devFile *os.File
	if runtime.GOOS == "linux" {
		if !isBlockDevice {
			log.Error().Msgf("%s must be a block device", dev)
			return
		}
		if strings.HasPrefix("/dev/dm-", dev) {
			log.Error().Msgf("%s is not static enough a name to allow. please use a /dev/mapper/<name> or /dev/by-<attr>/<value> dev path", dev)
			return
		}
		if strings.HasPrefix("/dev/sd", dev) {
			log.Error().Msgf("%s is not a static name. using a /dev/mapper/<name> or /dev/by-<attr>/<value> dev path is safer", dev)
			return
		}
		log.Info().Msgf("using directio on block device %s", dev)
		if devFile, err = directio.OpenFile(dev, os.O_RDWR|os.O_SYNC|syscall.O_DSYNC, 0755); err != nil {
			log.Error().Msgf("%s must be a block device", dev)
			return
		}
	} else {
		if isCharDevice {
			log.Info().Msgf("using char device %s", dev)
			return
		} else {
			log.Error().Msgf("%s must be a block device", dev)
			return
		}
		log.Info().Msgf("using char device %s", dev)
		if devFile, err = os.OpenFile(dev, os.O_RDWR, 0755); err != nil {
			log.Error().Msgf("%s must be a block device", dev)
			return
		}
	}

	nodes := t.GetStrings("nodes")
	if len(nodes) == 0 {
		k := key.T{Section: "cluster", Option: "nodes"}
		nodes = t.Config().GetStrings(k)
	}
	oNodes := hostname.OtherNodes(nodes)
	log.Debug().Msgf("configure %s, timeout=%s interval=%s dev=%s nodes=%s onodes=%s", t.Name(), timeout, interval, dev, nodes, oNodes)
	t.SetNodes(oNodes)
	t.SetTimeout(timeout)
	name := t.Name()
	baba := base{
		log: log,
		device: device{
			file: devFile,
		},
	}
	if err := baba.LoadPeerConfig(t.Nodes()); err != nil {
		log.Error().Err(err).Msgf("configure: load peer configs")
		return
	}
	tx := newTx(ctx, name, oNodes, baba, timeout, interval)
	t.SetTx(tx)
	rx := newRx(ctx, name, oNodes, baba, timeout, interval)
	t.SetRx(rx)
}

// SlotOffset returns the offset of the meta page of the slot.
func (t device) MetaSlotOffset(slot int) int64 {
	return int64(slot) * int64(PageSize)
}

func (t device) ReadMetaSlot(slot int) ([]byte, error) {
	offset := t.MetaSlotOffset(slot)
	if _, err := t.file.Seek(offset, os.SEEK_SET); err != nil {
		return nil, err
	}
	block := directio.AlignedBlock(PageSize)
	if _, err := io.ReadFull(t.file, block); err != nil {
		return nil, err
	}
	return block, nil
}

func (t device) WriteMetaSlot(slot int, b []byte) error {
	if len(b) > PageSize {
		return errors.Errorf("attempt to write too long data in meta slot %d", slot)
	}
	offset := t.MetaSlotOffset(slot)
	if _, err := t.file.Seek(offset, os.SEEK_SET); err != nil {
		return err
	}
	block := directio.AlignedBlock(PageSize)
	copy(block, b)
	_, err := t.file.Write(block)
	return err
}

func (t device) DataSlotOffset(slot int) int64 {
	return int64(MetaSize) + int64(slot)*int64(SlotSize)
}

func (t device) ReadDataSlot(slot int) ([]byte, error) {
	offset := t.DataSlotOffset(slot)
	if _, err := t.file.Seek(offset, os.SEEK_SET); err != nil {
		return nil, err
	}
	data := make([]byte, 0)
	totalRead := 0
	for {
		block := directio.AlignedBlock(PageSize)
		n, err := io.ReadFull(t.file, block)
		totalRead += n
		if err != nil {
			return data, err
		}
		if n == 0 {
			break
		}
		i := bytes.IndexRune(block, '\x00')
		if i < 0 {
			data = append(data, block...)
		} else {
			data = append(data, block[:i]...)
			break
		}
		if totalRead >= SlotSize {
			break
		}
	}
	return data, nil
}

func (t device) WriteDataSlot(slot int, b []byte) error {
	b = append(b, []byte{'\x00'}...)
	if len(b) > SlotSize {
		return errors.Errorf("attempt to write too long data in data slot %d", slot)
	}
	offset := t.DataSlotOffset(slot)
	if _, err := t.file.Seek(offset, os.SEEK_SET); err != nil {
		return err
	}
	remaining := len(b)
	for {
		if remaining == 0 {
			break
		}
		block := directio.AlignedBlock(PageSize)
		copied := copy(block, b)
		if _, err := t.file.Write(block); err != nil {
			return err
		}
		if copied < PageSize {
			return nil
		}
		b = b[copied:]
		remaining -= copied
	}
	return nil
}

func (t *base) LoadPeerConfig(nodes []string) error {
	var errs error
	t.peerConfigs = make(peerConfigs)
	for _, node := range append(nodes, hostname.Hostname()) {
		t.peerConfigs[node] = newPeerConfig()
	}
	for slot := 0; slot < MaxSlots; slot += 1 {
		b, err := t.device.ReadMetaSlot(slot)
		if err != nil {
			errs := xerrors.Append(errs, err)
			return errs
		}
		nodename := string(b[:bytes.IndexRune(b, '\x00')])
		data, ok := t.peerConfigs[nodename]
		if !ok {
			// foreign node
			continue
		}
		if data.Slot > 0 && data.Slot != slot {
			errs = xerrors.Append(errs, errors.Errorf("duplicate slot %d for node %s (first %d)", slot, nodename, data.Slot))
			continue
		}
		t.log.Info().Msgf("detect slot %d for node %s", slot, nodename)
		data.Slot = slot
		t.peerConfigs[nodename] = data
	}
	return errs
}

func (t *base) AllocateSlot(nodename string) (peerConfig, error) {
	conf := newPeerConfig()
	conf.Slot = t.peerConfigs.FreeSlot()
	if conf.Slot < 0 {
		return conf, errors.New("no free slot on dev")
	}
	b := []byte(nodename)
	b = append(b, '\x00')
	if err := t.WriteMetaSlot(conf.Slot, b); err != nil {
		return conf, err
	}
	t.peerConfigs[nodename] = conf
	return conf, nil
}

func (t *base) GetPeer(s string) (peerConfig, error) {
	data := t.peerConfigs.Get(s)
	if data.Slot >= 0 {
		return data, nil
	}
	return t.AllocateSlot(s)
}

func (t peerConfigs) Set(s string, data peerConfig) {
	t[s] = data
}

func (t peerConfigs) Get(s string) peerConfig {
	if data, ok := t[s]; ok {
		return data
	} else {
		return newPeerConfig()
	}
}

func (t peerConfigs) UsedSlots() map[int]any {
	m := make(map[int]any)
	for _, data := range t {
		m[data.Slot] = nil
	}
	return m
}

func (t peerConfigs) FreeSlot() int {
	used := t.UsedSlots()
	for slot := 0; slot < MaxSlots; slot += 1 {
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
