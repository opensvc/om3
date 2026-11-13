//go:build linux

package resdiskvg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// What a group hands out of a physical volume is the inverse of what it asks
// of one: asked for what it hands out, it asks no more than the volume holds,
// and given what it asks, it hands out at least what was asked.
func TestAGroupHandsOutWhatItAsksFor(t *testing.T) {
	const m = int64(1024 * 1024)
	for _, tc := range []struct {
		name             string
		overhead, extent int64
	}{
		{"1m in front, 4m extents", m, 4 * m},
		{"4m kept, the slack of a 128m volume", 4 * m, 4 * m},
		{"no extent known", m, 0},
		{"odd sizes", 1536 * 1024, 32 * m},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, x := range []int64{8 * m, 128 * m, 130 * m, 172*m + 4096, 1000000007} {
				out := vgHandsOut(x, tc.overhead, tc.extent)
				assert.LessOrEqual(t, vgAsks(out, tc.overhead, tc.extent), x+511, "asking for what it hands out of %d", x)
				assert.GreaterOrEqual(t, vgHandsOut(vgAsks(x, tc.overhead, tc.extent), tc.overhead, tc.extent), x, "handing out of what it asks for %d", x)
			}
		})
	}
	assert.Equal(t, 124*m, vgHandsOut(128*m, 4*m, 4*m), "the group of a 128m loop file")
	assert.Equal(t, 168*m, vgHandsOut(172*m, 4*m, 4*m), "grown to 172m")
	assert.Equal(t, 172*m, vgAsks(168*m, 4*m, 4*m), "and back")
	assert.Equal(t, int64(0), vgHandsOut(m, 4*m, 4*m), "a volume smaller than what the group keeps")
}
