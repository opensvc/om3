package netstatus

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveLoadRemove(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "network")
	m, err := Load(dir)
	require.NoError(t, err, "absent is no error")
	assert.Empty(t, m)

	st := Status{
		Type:      "routed_bridge",
		Network:   "10.100.0.0/16",
		Dev:       "obr_rb",
		Subnet:    "10.100.0.0/24",
		Peers:     []Peer{{Nodename: "n2", Local: net.ParseIP("10.0.0.1"), Remote: net.ParseIP("10.0.0.2"), Subnet: "10.100.1.0/24", Tunnel: "tun10002"}},
		AppliedAt: time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC),
	}
	require.NoError(t, Save(dir, "rb", st))
	// The state file of a driver in the same directory is no status.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "l2.l2_bridge.json"), []byte("{}"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.status.json"), []byte("{"), 0o600))

	m, err = Load(dir)
	require.NoError(t, err)
	require.Len(t, m, 1)
	got := m["rb"]
	assert.Equal(t, "obr_rb", got.Dev)
	assert.True(t, got.Peers[0].Remote.Equal(net.ParseIP("10.0.0.2")))
	assert.True(t, got.AppliedAt.Equal(st.AppliedAt))

	require.NoError(t, Remove(dir, "rb"))
	require.NoError(t, Remove(dir, "rb"), "absent is no error")
	m, err = Load(dir)
	require.NoError(t, err)
	assert.Empty(t, m)
}

func TestDeepCopySharesNothing(t *testing.T) {
	st := &Status{Peers: []Peer{{Nodename: "n2", Remote: net.ParseIP("10.0.0.2")}}}
	c := st.DeepCopy()
	c.Peers[0].Nodename = "n3"
	c.Peers[0].Remote[15] = 9
	assert.Equal(t, "n2", st.Peers[0].Nodename)
	assert.Equal(t, "10.0.0.2", st.Peers[0].Remote.String())
	assert.Nil(t, (*Status)(nil).DeepCopy())
}

func TestDataByNode(t *testing.T) {
	d := NewData()
	d.Set("rb", "n1", &Status{Type: "routed_bridge"})
	d.Set("rb", "n2", &Status{Type: "routed_bridge"})
	d.Set("br", "n1", &Status{Type: "bridge"})
	assert.Len(t, d.GetAll(), 3)
	assert.Len(t, d.GetByNode("n1"), 2)
	d.Unset("rb", "n1")
	assert.Len(t, d.GetByNode("n1"), 1)
}
