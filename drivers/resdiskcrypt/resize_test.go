//go:build linux

package resdiskcrypt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// What a luks volume hands out of a device is the inverse of what it asks of
// one: asked for what it hands out, it asks no more than the device holds, and
// given what it asks, it hands out at least what was asked.
func TestALuksVolumeHandsOutWhatItAsksFor(t *testing.T) {
	const m = int64(1024 * 1024)
	for _, offset := range []int64{0, 2 * m, 16 * m, 4096} {
		for _, x := range []int64{32 * m, 128 * m, 172*m + 3, 1 << 40} {
			out := cryptHandsOut(x, offset)
			assert.LessOrEqual(t, cryptAsks(out, offset), x, "asking for what it hands out of %d", x)
			assert.Greater(t, cryptAsks(out+512, offset), x, "the most it hands out of %d, in sectors", x)
			y := x / 512 * 512
			assert.GreaterOrEqual(t, cryptHandsOut(cryptAsks(y, offset), offset), y, "handing out of what it asks for %d", y)
		}
	}
	assert.Equal(t, 112*m, cryptHandsOut(128*m, 16*m), "a device less the header")
	assert.Equal(t, int64(0), cryptHandsOut(m, 16*m), "a device smaller than the header")
}
