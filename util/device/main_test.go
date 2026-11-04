package device

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ZeroTail zeroes the end of a device, and leaves the rest as it was.
func TestZeroTailZeroesTheEndOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dev")
	require.NoError(t, os.WriteFile(p, bytes.Repeat([]byte{0xff}, 8192), 0o600))
	require.NoError(t, New(p).ZeroTail(4096))
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Len(t, b, 8192)
	assert.Equal(t, bytes.Repeat([]byte{0xff}, 4096), b[:4096])
	assert.Equal(t, make([]byte, 4096), b[4096:])
}

// A device smaller than the tail asked is zeroed whole, and does not grow.
func TestZeroTailOfASmallDeviceZeroesItWhole(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dev")
	require.NoError(t, os.WriteFile(p, bytes.Repeat([]byte{0xff}, 512), 0o600))
	require.NoError(t, New(p).ZeroTail(1<<20))
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, make([]byte, 512), b)
}
