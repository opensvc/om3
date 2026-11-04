package resdisklv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A volume made on the data of an origin, as a snapshot or a cache, has no
// end of its own to zero.
func TestAVolumeMadeFromAnOriginIsTold(t *testing.T) {
	for _, args := range [][]string{
		{"-s"},
		{"--snapshot", "vg/origin"},
		{"--type", "snapshot"},
		{"--type=cache"},
		{"-H", "--cachepool", "vg/pool"},
		{"--type", "writecache"},
	} {
		assert.True(t, createsFromAnOrigin(args), "%v", args)
	}
	for _, args := range [][]string{
		nil,
		{"--type", "raid1", "-m", "1"},
		{"--type=linear"},
		{"-i", "2", "-I", "64"},
	} {
		assert.False(t, createsFromAnOrigin(args), "%v", args)
	}
}
