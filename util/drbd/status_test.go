package drbd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStatus(t *testing.T) {
	b, err := os.ReadFile("testdata/status-synctarget.json")
	require.NoError(t, err)
	st, err := ParseStatus(b)
	require.NoError(t, err)
	assert.Equal(t, "c33kvm1.root.vol.cluster33", st.Name)
	assert.False(t, st.IsUpToDate(), "a resync target is not up to date")
	assert.Equal(t, "Inconsistent", st.DiskStates())
	assert.True(t, st.HasConnected())
	assert.True(t, st.HasUpToDatePeer())
	assert.Equal(t, uint64(2048), st.OutOfSync())
	assert.Equal(t, "labu24c33n1 Connected SyncTarget UpToDate", st.Peers())

	st.Devices[0].DiskState = DiskStateUpToDate
	assert.True(t, st.IsUpToDate())
	st.Connections[0].ConnectionState = ConnStateStandAlone
	assert.False(t, st.HasConnected())
	assert.False(t, st.HasUpToDatePeer(), "a peer not connected is no source")

	_, err = ParseStatus([]byte("[]"))
	assert.Error(t, err)
}
