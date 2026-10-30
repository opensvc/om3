package reqh2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVsockURL(t *testing.T) {
	cid, port, err := parseVsockURL("vsock://3:1215")
	require.NoError(t, err)
	assert.Equal(t, uint32(3), cid)
	assert.Equal(t, uint32(1215), port)

	for _, s := range []string{"vsock://3", "vsock://a:1215", "vsock://3:b", "vsock://:1215", "vsock://3:"} {
		_, _, err := parseVsockURL(s)
		assert.Error(t, err, s)
	}
}
