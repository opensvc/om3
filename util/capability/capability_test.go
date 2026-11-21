package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A bounding set lists the capabilities kept, or, prefixed with ~, the ones
// dropped, in any case, a duplicate taken once; an unknown name is refused.
func TestParseBoundingSet(t *testing.T) {
	bs, err := ParseBoundingSet("cap_net_bind_service CAP_CHOWN CAP_CHOWN")
	require.NoError(t, err)
	assert.False(t, bs.Drop)
	assert.Equal(t, []int{0, 10}, bs.Caps)
	assert.True(t, bs.Keeps(10), "CAP_NET_BIND_SERVICE kept")
	assert.False(t, bs.Keeps(21), "CAP_SYS_ADMIN dropped")
	assert.Equal(t, "CAP_CHOWN CAP_NET_BIND_SERVICE", bs.String())

	bs, err = ParseBoundingSet(" ~CAP_SYS_ADMIN CAP_SYS_MODULE ")
	require.NoError(t, err)
	assert.True(t, bs.Drop)
	assert.False(t, bs.Keeps(21))
	assert.True(t, bs.Keeps(0), "CAP_CHOWN kept: not listed in a drop list")
	assert.Equal(t, "~CAP_SYS_MODULE CAP_SYS_ADMIN", bs.String())

	bs, err = ParseBoundingSet("")
	require.NoError(t, err)
	assert.False(t, bs.Keeps(0), "the empty set kept: every capability dropped")

	_, err = ParseBoundingSet("CAP_CHOWN CAP_SYS_ADMN net_admin")
	assert.ErrorContains(t, err, "unknown capabilities: CAP_SYS_ADMN net_admin")
}

func TestName(t *testing.T) {
	assert.Equal(t, "CAP_CHECKPOINT_RESTORE", Name(40))
	assert.Equal(t, "cap_41", Name(41), "a capability of a newer kernel")
	n, ok := Number("cap_bpf")
	assert.True(t, ok)
	assert.Equal(t, 39, n)
}
