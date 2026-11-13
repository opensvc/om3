//go:build linux

package resdiskdrbd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// What a drbd resource hands out of a device is the inverse of what it asks of
// one: asked for what it hands out, it asks no more than the device holds, and
// given what it asks, it hands out at least what was asked.
func TestADrbdHandsOutWhatItAsksFor(t *testing.T) {
	const m = int64(1024 * 1024)
	for _, peers := range []int64{0, 1, 3, 7, 31} {
		for _, x := range []int64{5 * m, 128 * m, 168 * m, 168*m + 7, 1 << 40} {
			out := drbdHandsOut(x, peers)
			assert.LessOrEqual(t, drbdAsks(out, peers), x, "asking for what it hands out of %d with %d peers", x, peers)
			assert.Greater(t, drbdAsks(out+1, peers), x, "the most it hands out of %d with %d peers", x, peers)
			assert.GreaterOrEqual(t, drbdHandsOut(drbdAsks(x, peers), peers), x, "handing out of what it asks for %d with %d peers", x, peers)
		}
	}
	assert.Equal(t, int64(0), drbdHandsOut(4*m, 1), "a device holding no more than the metadata")
	assert.Equal(t, int64(0), drbdHandsOut(0, 1))

	// The logical volume of a 172m drbd pool volume is 168m: the filesystem
	// is asked for what that hands out, and asks the volume no more than it.
	out := drbdHandsOut(168*m, 1)
	assert.Less(t, out, 164*m)
	assert.LessOrEqual(t, drbdAsks(out, 1), 168*m)
}
