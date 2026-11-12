package hbdisk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/sign"
)

// A disk wiped under a running stream suspends it, rather than have it write
// on a disk another cluster may be signed for, and signed again resumes it. A
// signature not read says nothing, and changes nothing.
func TestCheckSignature(t *testing.T) {
	const maxSlots = 8
	dev := filepath.Join(t.TempDir(), "disk")
	f, err := os.OpenFile(dev, os.O_RDWR|os.O_CREATE, 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	require.NoError(t, f.Truncate(metaSize(maxSlots)+maxSlots*sign.SlotSizeInt64))
	require.NoError(t, sign.Sign(dev, sign.SignOptions{Force: true}))
	b := base{
		device: device{path: dev, file: f, metaSize: metaSize(maxSlots)},
		log:    plog.NewDefaultLogger(),
	}

	assert.False(t, b.checkSignature(), "signed: no change")
	assert.False(t, b.invalidSig)

	require.NoError(t, sign.Wipe(dev, 0))
	assert.True(t, b.checkSignature(), "wiped: suspended")
	assert.True(t, b.invalidSig)
	assert.False(t, b.checkSignature(), "still wiped: no change")

	require.NoError(t, sign.Sign(dev, sign.SignOptions{Force: true}))
	assert.True(t, b.checkSignature(), "signed again: resumed")
	assert.False(t, b.invalidSig)

	b.invalidSig = true
	require.NoError(t, f.Close())
	assert.False(t, b.checkSignature(), "a read failing changes nothing")
	assert.True(t, b.invalidSig)

	assert.Equal(t, 4*time.Second, signatureCheckPeriod(4*time.Second, true), "suspended: checked every interval")
	assert.Equal(t, 12*time.Second, signatureCheckPeriod(4*time.Second, false), "signed: checked every 3 intervals")
}

// A stream starts on a disk not signed, suspended until it is, and on a disk
// of the former signature. Another failure of the open fails the start.
func TestStartWith(t *testing.T) {
	b := base{log: plog.NewDefaultLogger()}
	require.NoError(t, b.startWith(fmt.Errorf("verify: %w", sign.ErrLegacySignature)))
	assert.False(t, b.invalidSig, "a legacy signature beats")

	require.NoError(t, b.startWith(fmt.Errorf("verify: %w", sign.ErrWrongSignature)))
	assert.True(t, b.invalidSig, "not signed: started suspended")

	b = base{log: plog.NewDefaultLogger()}
	require.NoError(t, b.startWith(fmt.Errorf("verify: %w", sign.ErrChecksumMismatch)))
	assert.True(t, b.invalidSig, "a header corrupted: started suspended")

	b = base{log: plog.NewDefaultLogger()}
	assert.Error(t, b.startWith(errors.New("no such device")))
	assert.False(t, b.invalidSig)
}

// A stream restarted on a signed disk finds the slots held there: the
// transmitter writes in the slot of its node again, rather than take another,
// and the receiver reads the slots of its peers. On a disk not signed, the
// scan waits for the signature.
func TestInitialScan(t *testing.T) {
	const maxSlots = 8
	dev := filepath.Join(t.TempDir(), "disk")
	f, err := os.OpenFile(dev, os.O_RDWR|os.O_CREATE, 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	require.NoError(t, f.Truncate(metaSize(maxSlots)+maxSlots*sign.SlotSizeInt64))
	require.NoError(t, sign.Sign(dev, sign.SignOptions{Force: true}))
	newBase := func(localhost string) base {
		return base{
			device:    device{path: dev, file: f, metaSize: metaSize(maxSlots)},
			log:       plog.NewDefaultLogger(),
			maxSlots:  maxSlots,
			localhost: localhost,
		}
	}

	first := &tx{base: newBase("n1"), log: plog.NewDefaultLogger()}
	require.NoError(t, first.allocateSlot())
	require.GreaterOrEqual(t, first.slot, minimumSlot)

	restarted := &tx{base: newBase("n1"), log: plog.NewDefaultLogger()}
	restarted.claimSlot()
	assert.Equal(t, first.slot, restarted.slot, "the transmitter finds the slot of its node")

	r := &rx{base: newBase("n2"), nodes: []string{"n1"}}
	require.NoError(t, r.initialScan())
	assert.Equal(t, first.slot, r.base.nodeSlot["n1"], "the receiver finds the slot of its peer")

	suspended := &tx{base: newBase("n1"), log: plog.NewDefaultLogger()}
	suspended.base.invalidSig = true
	suspended.claimSlot()
	assert.Equal(t, 0, suspended.slot, "not signed: no slot yet")

	r = &rx{base: newBase("n2"), nodes: []string{"n1"}}
	r.base.invalidSig = true
	require.NoError(t, r.initialScan())
	assert.Empty(t, r.base.nodeSlot, "not signed: not scanned")
}

// signedDisk returns a signed heartbeat disk of maxSlots slots in a file, and
// a function returning a base on it for a node.
func signedDisk(t *testing.T, maxSlots int) func(localhost string) base {
	t.Helper()
	dev := filepath.Join(t.TempDir(), "disk")
	f, err := os.OpenFile(dev, os.O_RDWR|os.O_CREATE, 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	require.NoError(t, f.Truncate(metaSize(maxSlots)+int64(maxSlots)*sign.SlotSizeInt64))
	require.NoError(t, sign.Sign(dev, sign.SignOptions{Force: true}))
	return func(localhost string) base {
		return base{
			device:    device{path: dev, file: f, metaSize: metaSize(maxSlots)},
			log:       plog.NewDefaultLogger(),
			maxSlots:  maxSlots,
			localhost: localhost,
		}
	}
}

// slotsOf returns the metadata slots held by nodename.
func slotsOf(t *testing.T, b base, nodename string) []int {
	t.Helper()
	l := make([]int, 0)
	for slot := minimumSlot; slot < b.maxSlots; slot++ {
		meta, err := b.device.readMetaSlot(slot)
		require.NoError(t, err)
		if nodeFromMetadata(meta) == nodename {
			l = append(l, slot)
		}
	}
	return l
}

// drain returns the commands sent on cmdC.
func drain(cmdC chan any) []any {
	l := make([]any, 0)
	for {
		select {
		case c := <-cmdC:
			l = append(l, c)
		default:
			return l
		}
	}
}

// A transmitter started on a disk not signed, resumed once the disk is
// signed, writes in the slot its node holds there, as a wipe and a sign keep
// the metadata: a new slot would leave the node twice, its peers reading the
// slot found first, which nobody writes. A node holding no slot gets one.
func TestResumeWritesTheSlotHeld(t *testing.T) {
	newBase := signedDisk(t, 8)
	first := &tx{base: newBase("n1"), log: plog.NewDefaultLogger()}
	require.NoError(t, first.allocateSlot())
	held := first.slot

	for _, nodename := range []string{"n1", "n2"} {
		cmdC := make(chan any, 16)
		resumed := &tx{base: newBase(nodename), log: plog.NewDefaultLogger(), id: "hb#1.tx", nodes: []string{"n3"}, cmdC: cmdC}
		resumed.base.invalidSig = true
		resumed.claimSlot()
		require.Equal(t, 0, resumed.slot, "%s: started suspended, no slot", nodename)

		resumed.base.invalidSig = false
		resumed.onSignatureChange()
		require.GreaterOrEqual(t, resumed.slot, minimumSlot, nodename)
		assert.Equal(t, []int{resumed.slot}, slotsOf(t, resumed.base, nodename), "%s is once in the metadata", nodename)
		if nodename == "n1" {
			assert.Equal(t, held, resumed.slot, "n1 writes in the slot it holds")
		}
		assert.Contains(t, drain(cmdC), any(hbctrl.CmdSetState{ID: "hb#1.tx", State: "running"}))
	}
}

// A stream suspended stops beating at once, for each of its peers, rather
// than one timeout later, and a receiver resumed says nothing of its peers
// before it reads a frame of theirs.
func TestSuspensionStopsBeating(t *testing.T) {
	newBase := signedDisk(t, 8)
	nodes := []string{"n2", "n3"}
	notBeating := func(id string) []any {
		return []any{
			hbctrl.CmdSetPeerSuccess{Nodename: "n2", HbID: id, Success: false},
			hbctrl.CmdSetPeerSuccess{Nodename: "n3", HbID: id, Success: false},
		}
	}

	txC := make(chan any, 16)
	tr := &tx{base: newBase("n1"), log: plog.NewDefaultLogger(), id: "hb#1.tx", nodes: nodes, cmdC: txC}
	tr.base.invalidSig = true
	tr.onSignatureChange()
	cmds := drain(txC)
	assert.Contains(t, cmds, any(hbctrl.CmdSetState{ID: "hb#1.tx", State: "failed"}))
	assert.Subset(t, cmds, notBeating("hb#1.tx"))

	rxC := make(chan any, 16)
	r := &rx{base: newBase("n1"), log: plog.NewDefaultLogger(), id: "hb#1.rx", nodes: nodes, cmdC: rxC}
	r.base.invalidSig = true
	r.onSignatureChange()
	cmds = drain(rxC)
	assert.Contains(t, cmds, any(hbctrl.CmdSetState{ID: "hb#1.rx", State: "failed"}))
	assert.Subset(t, cmds, notBeating("hb#1.rx"))

	r.base.invalidSig = false
	r.onSignatureChange()
	for _, c := range drain(rxC) {
		_, ok := c.(hbctrl.CmdSetPeerSuccess)
		assert.False(t, ok, "resumed: no peer beats before a frame is read: %#v", c)
	}
}
