package filesystems

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The size a superblock says is its block count by its block size.
func TestParseExtSuperblockSize(t *testing.T) {
	out := `dumpe2fs 1.46.5 (30-Dec-2021)
Filesystem volume name:   <none>
Block count:              32768
Reserved block count:     1638
Block size:               4096
Fragment size:            4096
`
	n, err := parseExtSuperblockSize(out)
	require.NoError(t, err)
	assert.Equal(t, int64(128*1024*1024), n)
	_, err = parseExtSuperblockSize("dumpe2fs 1.46.5 (30-Dec-2021)\n")
	assert.Error(t, err)
}
