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
