package ipam

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recordsByIP(t *testing.T, dir string) map[string]PeerRecord {
	t.Helper()
	l, err := ReadPeerRecords(dir)
	require.NoError(t, err)
	m := make(map[string]PeerRecord)
	for _, r := range l {
		m[r.IP.String()] = r
	}
	return m
}

// The records of an object on a node are what its last status reported: the
// addresses it no longer reports are removed, the others of the node and the
// ones of the other nodes left alone.
func TestSetPeerRecords(t *testing.T) {
	dir := t.TempDir()
	written, removed, err := SetPeerRecords(dir, "n2", "svc1", map[string]string{
		"fd00::1": "svc1!ip#0",
		"fd00::2": "svc1!ip#1",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, written)
	assert.Equal(t, 0, removed)
	_, _, err = SetPeerRecords(dir, "n3", "svc1", map[string]string{"fd00::3": "svc1!ip#0"})
	require.NoError(t, err)
	_, _, err = SetPeerRecords(dir, "n2", "svc2", map[string]string{"fd00::4": "svc2!ip#0"})
	require.NoError(t, err)

	written, removed, err = SetPeerRecords(dir, "n2", "svc1", map[string]string{"fd00::1": "svc1!ip#0"})
	require.NoError(t, err)
	assert.Equal(t, 0, written, "a record unchanged is not written again")
	assert.Equal(t, 1, removed)

	m := recordsByIP(t, dir)
	assert.Len(t, m, 3)
	assert.Equal(t, PeerRecord{IP: net.ParseIP("fd00::1"), Node: "n2", Key: "svc1!ip#0"}, m["fd00::1"])
	assert.Equal(t, "n3", m["fd00::3"].Node)
	assert.Equal(t, "n2", m["fd00::4"].Node)

	_, removed, err = SetPeerRecords(dir, "n2", "svc1", nil)
	require.NoError(t, err)
	assert.Equal(t, 1, removed, "an object reporting nothing holds nothing")
	assert.Len(t, recordsByIP(t, dir), 2)

	n, err := DropPeerRecords(dir, func(r PeerRecord) bool { return r.Node != "n3" })
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Len(t, recordsByIP(t, dir), 1)
}

// An address another node reported holding is not drawn here.
func TestAllocateKeepsClearOfThePeerRecords(t *testing.T) {
	_, rng, err := net.ParseCIDR("fd00::/126")
	require.NoError(t, err)
	i := &T{Name: "lan", Range: rng, Dir: t.TempDir(), ClusterDir: t.TempDir(), ClusterWide: true}
	_, _, err = SetPeerRecords(i.ClusterDir, "n2", "other", map[string]string{"fd00::1": "other!ip#0", "fd00::2": "other!ip#1"})
	require.NoError(t, err)
	ip, err := i.Allocate("svc1!ip#0")
	require.NoError(t, err)
	assert.Equal(t, "fd00::3", ip.String(), "the only address no peer holds")
}
