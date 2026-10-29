package hbdisk

import (
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
	assert.Equal(t, time.Minute, signatureCheckPeriod(4*time.Second, false))
}
