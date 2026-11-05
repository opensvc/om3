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
	require.NoError(t, sign.CreateAndFillDisk(dev))
	b := base{
		device: device{path: dev, file: f, metaSize: metaSize(maxSlots)},
		log:    plog.NewDefaultLogger(),
	}

	assert.False(t, b.checkSignature(), "signed: no change")
	assert.False(t, b.invalidSig)

	require.NoError(t, sign.RemoveHeaderFromDisk(dev))
	assert.True(t, b.checkSignature(), "wiped: suspended")
	assert.True(t, b.invalidSig)
	assert.False(t, b.checkSignature(), "still wiped: no change")

	require.NoError(t, sign.CreateAndFillDisk(dev))
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
	require.NoError(t, sign.CreateAndFillDisk(dev))
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
	restarted.initialScan()
	assert.Equal(t, first.slot, restarted.slot, "the transmitter finds the slot of its node")

	r := &rx{base: newBase("n2"), nodes: []string{"n1"}}
	require.NoError(t, r.initialScan())
	assert.Equal(t, first.slot, r.base.nodeSlot["n1"], "the receiver finds the slot of its peer")

	suspended := &tx{base: newBase("n1"), log: plog.NewDefaultLogger()}
	suspended.base.invalidSig = true
	suspended.initialScan()
	assert.Equal(t, 0, suspended.slot, "not signed: no slot yet")

	r = &rx{base: newBase("n2"), nodes: []string{"n1"}}
	r.base.invalidSig = true
	require.NoError(t, r.initialScan())
	assert.Empty(t, r.base.nodeSlot, "not signed: not scanned")
}
