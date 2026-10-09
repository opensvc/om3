package hbdisk

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/sign"
)

// A wipe from a cluster sharing a disk with another took the signature from
// under the heartbeats of the other. The nodes beating on the disk, outside
// the ones given, are the ones writing their data slot while it is watched.
func TestBeatingForeignNodes(t *testing.T) {
	const maxSlots = 8
	dev := filepath.Join(t.TempDir(), "disk")
	f, err := os.OpenFile(dev, os.O_RDWR|os.O_CREATE, 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	require.NoError(t, f.Truncate(metaSize(maxSlots)+maxSlots*sign.SlotSizeInt64))
	d := device{path: dev, file: f, metaSize: metaSize(maxSlots)}

	slots := map[int]string{
		1: "n1",    // of this heartbeat, beating
		2: "other", // of another cluster, beating
		3: "gone",  // of another cluster, stopped
		4: "never", // of another cluster, never written
	}
	for slot, name := range slots {
		require.NoError(t, d.writeMetaSlot(slot, append([]byte(name), endOfDataMarker)))
	}
	beat := func() error {
		if err := d.writeDataSlot(1, []byte("msg")); err != nil {
			return err
		}
		return d.writeDataSlot(2, []byte("msg"))
	}
	require.NoError(t, beat())
	require.NoError(t, d.writeDataSlot(3, []byte("msg")))

	// Beat all along the window, as a live node does. A single beat at a fixed
	// delay could land before the first read on a slow runner, and show no
	// change: whenever the first read happens, a later beat follows it.
	window := 300 * time.Millisecond
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(window / 10)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := beat(); err != nil {
					// require would stop a goroutine that is not the test one.
					t.Error(err)
					return
				}
			}
		}
	}()
	beating, err := BeatingForeignNodes(context.Background(), dev, maxSlots, []string{"n1", "n2"}, window)
	close(done)
	<-stopped
	require.NoError(t, err)
	require.Equal(t, []string{"other"}, beating)
}

// A disk no other cluster writes to is answered at once.
func TestBeatingForeignNodesNone(t *testing.T) {
	const maxSlots = 8
	dev := filepath.Join(t.TempDir(), "disk")
	f, err := os.OpenFile(dev, os.O_RDWR|os.O_CREATE, 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	require.NoError(t, f.Truncate(metaSize(maxSlots)+maxSlots*sign.SlotSizeInt64))
	d := device{path: dev, file: f, metaSize: metaSize(maxSlots)}
	require.NoError(t, d.writeMetaSlot(1, append([]byte("n1"), endOfDataMarker)))

	begin := time.Now()
	beating, err := BeatingForeignNodes(context.Background(), dev, maxSlots, []string{"n1", "n2"}, time.Hour)
	require.NoError(t, err)
	require.Empty(t, beating)
	require.Less(t, time.Since(begin), time.Second)
}
