package hbdisk

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/sign"
)

// sizedDisk returns a heartbeat disk of size bytes in a file, signed over
// what it holds, and a function returning a base of maxSlots slots on it for
// a node.
func sizedDisk(t *testing.T, size int64, maxSlots int, pages map[int]string) func(localhost string) base {
	t.Helper()
	dev := filepath.Join(t.TempDir(), "disk")
	f, err := os.OpenFile(dev, os.O_RDWR|os.O_CREATE, 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	require.NoError(t, f.Truncate(size))
	// a sign keeping the metadata, as on a disk signed before the sign
	// zeroed it
	require.NoError(t, sign.Sign(dev, sign.SignOptions{Force: true}))
	for slot, content := range pages {
		_, err := f.WriteAt([]byte(content), int64(slot)*sign.PageSizeInt64)
		require.NoError(t, err)
	}
	return func(localhost string) base {
		return base{
			device:    device{path: dev, file: f, metaSize: metaSize(maxSlots)},
			log:       plog.NewDefaultLogger(),
			maxSlots:  maxSlots,
			localhost: localhost,
		}
	}
}

const mib = 1024 * 1024

// The card 267 disk: 16 MiB, max_slots 1024, signed over what an ext4 left
// in its metadata area, with names at slots 34 and 35, past its end.
func card267Disk(t *testing.T) func(string) base {
	pages := map[int]string{
		1:  "\x04\x00\x00\x00\x01",
		2:  "\b\x00\x00",
		33: "\xff\x07\x00",
		34: "n2\x00",
		35: "n1\x00",
	}
	for slot := 3; slot < 33; slot++ {
		pages[slot] = "\x01\x02\x03"
	}
	return sizedDisk(t, 16*mib, 1024, pages)
}

// The slots of a node are the ones whose data slot fits on the device: 11 on
// a 16 MiB disk after 4 MiB of metadata and the unused slot 0. They were
// bounded by max_slots alone, and a node claimed slot 35, 39 MiB into a 16
// MiB disk, every write failing.
func TestSlotBound(t *testing.T) {
	b := card267Disk(t)("n1")
	assert.Equal(t, 12, b.device.slotBound(1024))
	assert.Equal(t, 8, b.device.slotBound(8), "max_slots bounds a larger disk")
	assert.Equal(t, 64, (&device{metaSize: metaSize(1024)}).slotBound(64), "no size: max_slots")
}

// A page holding what a filesystem left names no node, and is a free slot:
// a node claiming on a disk signed over leftovers takes slot 1, and its
// peer the next.
func TestAPageNamingNoNodeIsFree(t *testing.T) {
	newBase := card267Disk(t)
	n1 := &tx{base: newBase("n1"), log: plog.NewDefaultLogger()}
	n1.claimSlot()
	assert.Equal(t, 1, n1.slot)
	n2 := &tx{base: newBase("n2"), log: plog.NewDefaultLogger()}
	n2.claimSlot()
	assert.Equal(t, 2, n2.slot)

	r := &rx{base: newBase("n2"), nodes: []string{"n1"}}
	require.NoError(t, r.initialScan())
	assert.Equal(t, 1, r.base.nodeSlot["n1"], "the receiver reads n1 at the slot it claimed, not past the end")
}

// A node holding a slot past the end of the device claims one fitting, and
// its page past the end is blanked: found again once the disk grows, it
// would name the node twice.
func TestASlotPastTheEndIsClaimedAgain(t *testing.T) {
	newBase := card267Disk(t)
	n1 := &tx{base: newBase("n1"), log: plog.NewDefaultLogger()}
	n1.claimSlot()
	assert.Less(t, n1.slot, 12)
	b, err := n1.base.readMetaSlot(35)
	require.NoError(t, err)
	assert.Empty(t, nodeFromMetadata(b), "the page past the end is blank")
	b, err = n1.base.readMetaSlot(34)
	require.NoError(t, err)
	assert.Equal(t, "n2", nodeFromMetadata(b), "the page of another node is left")
}

// A disk whose fitting slots are all held says so, rather than return a slot
// past its end.
func TestNoFreeSlotSaysHowManyFit(t *testing.T) {
	pages := make(map[int]string)
	for slot := 1; slot < 12; slot++ {
		pages[slot] = "other" + strings.Repeat("x", slot) + "\x00"
	}
	b := sizedDisk(t, 16*mib, 1024, pages)("n1")
	_, err := b.freeSlot()
	assert.ErrorContains(t, err, "no free slot among the 11 slots fitting on")
}

// A metadata page names a node when it holds a node name then zeroes to its
// end, the way a node writes it.
func TestNodeFromMetadata(t *testing.T) {
	page := func(s string) []byte {
		b := make([]byte, sign.PageSize)
		copy(b, s)
		return b
	}
	assert.Equal(t, "dev2n1", nodeFromMetadata(page("dev2n1\x00")))
	assert.Equal(t, "node-1.example_com", nodeFromMetadata(page("node-1.example_com")))
	assert.Empty(t, nodeFromMetadata(page("")), "blank")
	assert.Empty(t, nodeFromMetadata(page("\x04\x00\x00\x00\x01")), "a byte no name holds")
	assert.Empty(t, nodeFromMetadata(page("dev2n1\x00garbage")), "not zeroes to the end")
	assert.Empty(t, nodeFromMetadata(page("dev 2\x00")), "a space")
	assert.Empty(t, nodeFromMetadata([]byte("dev2n1")), "no end marker")
}

// A device smaller than the metadata area and a data slot per node after the
// unused slot 0 is refused: the start of its streams fails, saying so.
func TestCheckSize(t *testing.T) {
	b := sizedDisk(t, 16*mib, 1024, nil)("n1")
	require.NoError(t, b.device.checkSize(1024, 2))
	require.NoError(t, b.device.checkSize(1024, 11))
	err := b.device.checkSize(1024, 12)
	assert.ErrorContains(t, err, "is 16777216 bytes, under the 17825792 bytes of 1024 metadata pages")
	assert.NoError(t, (&device{}).checkSize(1024, 2), "a size unknown is not checked")
}

// A /dev/sd<x> dev path is warned of, as in v2, and a /dev/dm-<n> one is
// refused at open: both checks had their arguments swapped, and accepted any
// path.
func TestDevPathChecks(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the dev paths of linux")
	}
	assert.Contains(t, devPathWarning("/dev/sdb"), "not a static name")
	assert.Empty(t, devPathWarning("/dev/mapper/36001405bd4a1e00af6e434781564a425"))
	d := &device{path: "/dev/dm-18"}
	assert.ErrorContains(t, d.openDevice("/dev/dm-18"), "not static enough a name")
}

// commandsOf returns the commands of type C sent on cmdC.
func commandsOf[C any](l []any) []C {
	r := make([]C, 0)
	for _, c := range l {
		if v, ok := c.(C); ok {
			r = append(r, v)
		}
	}
	return r
}

// A transmitter failing to write its slot reports it once: the stream
// failed, an error alert saying why, and its peers not beating at once. It
// was logged at each interval, the stream listed running with no alert. A
// write working again reports the stream running, writing in its slot.
func TestATransmitterFailingToWriteSaysSo(t *testing.T) {
	newBase := sizedDisk(t, 16*mib, 8, nil)
	cmdC := make(chan any, 64)
	tr := &tx{base: newBase("n1"), log: plog.NewDefaultLogger(), id: "hb#4.tx", nodes: []string{"n2"}, cmdC: cmdC}
	require.NoError(t, tr.claimSlot())
	rw := tr.base.device.file
	ro, err := os.Open(rw.Name())
	require.NoError(t, err)
	defer func() { _ = ro.Close() }()

	tr.base.device.file = ro
	tr.send([]byte("frame"))
	cmds := drain(cmdC)
	assert.Contains(t, cmds, any(hbctrl.CmdSetState{ID: "hb#4.tx", State: "failed"}))
	assert.Contains(t, cmds, any(hbctrl.CmdSetPeerSuccess{Nodename: "n2", HbID: "hb#4.tx", Success: false}))
	alerts := commandsOf[hbctrl.CmdSetAlert](cmds)
	require.Len(t, alerts, 1)
	require.Len(t, alerts[0].Alert, 1)
	assert.Equal(t, "error", alerts[0].Alert[0].Severity)
	assert.Contains(t, alerts[0].Alert[0].Message, "write data slot 1")

	tr.send([]byte("frame"))
	assert.Empty(t, drain(cmdC), "the same failure is reported once")

	tr.base.device.file = rw
	tr.send([]byte("frame"))
	cmds = drain(cmdC)
	assert.Contains(t, cmds, any(hbctrl.CmdSetState{ID: "hb#4.tx", State: "running"}))
	assert.Contains(t, cmds, any(hbctrl.CmdSetPeerSuccess{Nodename: "n2", HbID: "hb#4.tx", Success: true}))
	assert.Contains(t, cmds, any(hbctrl.CmdSetDesc{HbID: "hb#4.tx", Nodename: "n2", Desc: "→ " + rw.Name() + " slot 1"}))
	alerts = commandsOf[hbctrl.CmdSetAlert](cmds)
	require.Len(t, alerts, 1)
	assert.Equal(t, "info", alerts[0].Alert[0].Severity, "the slot, as info")
}

// A transmitter that can not claim a slot reports it failed, with the reason
// as an error alert: it was a warning, the stream listed running.
func TestATransmitterWithNoSlotSaysSo(t *testing.T) {
	pages := make(map[int]string)
	for slot := 1; slot < 8; slot++ {
		pages[slot] = "other" + strings.Repeat("x", slot) + "\x00"
	}
	cmdC := make(chan any, 64)
	tr := &tx{base: sizedDisk(t, 16*mib, 8, pages)("n1"), log: plog.NewDefaultLogger(), id: "hb#4.tx", nodes: []string{"n2"}, cmdC: cmdC}
	tr.send([]byte("frame"))
	cmds := drain(cmdC)
	assert.Contains(t, cmds, any(hbctrl.CmdSetState{ID: "hb#4.tx", State: "failed"}))
	alerts := commandsOf[hbctrl.CmdSetAlert](cmds)
	require.Len(t, alerts, 1)
	assert.Equal(t, "error", alerts[0].Alert[0].Severity)
	assert.Contains(t, alerts[0].Alert[0].Message, "no free slot among the 7 slots fitting")
}
