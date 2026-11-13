//go:build linux

package resdiskmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// What an array hands out of its members is the inverse of what it asks of
// them: asked for what it hands out, it asks no more than each member holds,
// and given what it asks, it hands out at least what was asked.
func TestAnArrayHandsOutWhatItAsksFor(t *testing.T) {
	const m = int64(1024 * 1024)
	for _, tc := range []struct {
		name                      string
		dataDevs, chunk, overhead int64
	}{
		{"raid1", 1, 0, 2 * m},
		{"raid5 of 3, 512k chunks", 2, 512 * 1024, 2 * m},
		{"raid6 of 6, 64k chunks", 4, 64 * 1024, 4 * m},
		{"raid1, an odd overhead", 1, 0, 1536*1024 + 512},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, x := range []int64{8 * m, 128 * m, 172*m + 4096 + 3, 1 << 40} {
				out := mdHandsOut(x, tc.dataDevs, tc.chunk, tc.overhead)
				assert.LessOrEqual(t, mdAsks(out, tc.dataDevs, tc.chunk, tc.overhead), x, "asking for what it hands out of %d", x)
				assert.Greater(t, mdAsks(out+1, tc.dataDevs, tc.chunk, tc.overhead), x, "the most it hands out of %d", x)
				assert.GreaterOrEqual(t, mdHandsOut(mdAsks(x, tc.dataDevs, tc.chunk, tc.overhead), tc.dataDevs, tc.chunk, tc.overhead), x, "handing out of what it asks for %d", x)
			}
		})
	}
	assert.Equal(t, 126*m, mdHandsOut(128*m, 1, 0, 2*m), "a raid1 hands out a member less what it keeps on it")
	assert.Equal(t, int64(0), mdHandsOut(m, 1, 0, 2*m), "members holding no more than the array keeps")
}
