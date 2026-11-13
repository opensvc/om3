package resdisklv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A logical volume asks its volume group for what the group has taken, plus
// its growth in whole extents: a group with the free space holds it already.
func TestVGNeed(t *testing.T) {
	const m = int64(1024 * 1024)
	assert.Equal(t, 14*m, vgNeed(16*m, 4*m, 12*m, 14*m, m), "within the free space")
	assert.Equal(t, 20*m, vgNeed(16*m, 0, 8*m, 12*m, 4*m), "a group with no free space grows by the growth")
	assert.Equal(t, 16*m, vgNeed(16*m, 4*m, 12*m, 14*m, 4*m), "a growth of 2m takes one extent of 4m")
	assert.Equal(t, 16*m, vgNeed(16*m, 4*m, 12*m, 13*m, 4*m), "the growth rounded up to an extent")
	assert.Equal(t, 12*m, vgNeed(16*m, 4*m, 12*m, 10*m, 4*m), "no growth asks what is taken")
}

// What a volume can grow to in a group of a given size is the inverse of what
// it asks of its group: asked for what it can grow to, it asks no more than
// the group holds, and given what it asks, it can grow to at least what was
// asked.
func TestLVHandsOutWhatItAsksOfItsGroup(t *testing.T) {
	const m = int64(1024 * 1024)
	assert.Equal(t, 168*m, lvHandsOut(124*m, 0, 124*m, 168*m, 4*m), "the only volume of its group takes all of it")
	assert.Equal(t, 124*m, lvHandsOut(124*m, 0, 124*m, 124*m, 4*m), "a group that does not grow gives nothing more")
	assert.Equal(t, 16*m, lvHandsOut(16*m, 4*m, 12*m, 16*m, 4*m), "the free space of the group")
	assert.Equal(t, 8*m, lvHandsOut(16*m, 0, 12*m, 13*m, 4*m), "short of the group by 3m, an extent is given back")
	for _, tc := range []struct {
		name                     string
		size, free, from, extent int64
	}{
		{"alone in its group", 124 * m, 0, 124 * m, 4 * m},
		{"with free space", 16 * m, 4 * m, 12 * m, 4 * m},
		{"next to another volume", 64 * m, 8 * m, 16 * m, 4 * m},
		{"no extent known", 64 * m, 8 * m, 16 * m, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A volume is not shrunk, so its group is not asked for less than
			// what is taken: the inverse holds from there.
			used := tc.size - tc.free
			for _, x := range []int64{used, used + 44*m, used + 46*m + 512, used + 1000000000} {
				out := lvHandsOut(tc.size, tc.free, tc.from, x, tc.extent)
				assert.LessOrEqual(t, vgNeed(tc.size, tc.free, tc.from, out, tc.extent), x, "asking for what it hands out of %d", x)
				assert.GreaterOrEqual(t, lvHandsOut(tc.size, tc.free, tc.from, vgNeed(tc.size, tc.free, tc.from, x, tc.extent), tc.extent), x, "handing out of what it asks for %d", x)
			}
		})
	}
}
