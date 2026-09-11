//go:build linux

package netif

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBridge(t *testing.T) {
	// The loopback is on every Linux and is never a bridge.
	v, err := IsBridge("lo")
	require.NoError(t, err)
	assert.False(t, v)
}

// A device that is gone is not a bridge, and its absence is not an error: the
// caller is asking whether a carrier reading would mean anything, and for a
// device that is not there the answer is no either way.
func TestIsBridgeOfAMissingDeviceIsFalse(t *testing.T) {
	v, err := IsBridge("nosuchdev0")
	require.NoError(t, err)
	assert.False(t, v)
}
