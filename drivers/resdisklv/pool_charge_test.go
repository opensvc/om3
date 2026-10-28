package resdisklv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A thick volume is charged to its volume group, a thin one to its thin pool,
// named as a pool of thin volumes names its head, by the size it is seen
// with. A share of the group is no charge.
func TestPoolCharge(t *testing.T) {
	head, size := (&T{VGName: "data", Size: "1m"}).PoolCharge()
	assert.Equal(t, "data", head)
	assert.Equal(t, int64(1024*1024), size)

	head, size = (&T{VGName: "data", Size: "1m", ThinPool: "tp1"}).PoolCharge()
	assert.Equal(t, "data/tp1", head)
	assert.Equal(t, int64(1024*1024), size)

	head, _ = (&T{VGName: "data", Size: "100%FREE"}).PoolCharge()
	assert.Empty(t, head)
}
