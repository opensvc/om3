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
