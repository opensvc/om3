//go:build linux

package resdiskdrbd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/rawconfig"
	// The lo and default networks are implicit, and their drivers too, as
	// the daemon registers them.
	_ "github.com/opensvc/om3/v3/drivers/networkbridge"
	_ "github.com/opensvc/om3/v3/drivers/networklan"
	_ "github.com/opensvc/om3/v3/drivers/networklo"
	_ "github.com/opensvc/om3/v3/drivers/networkroutedbridge"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/drbd"
	"github.com/opensvc/om3/v3/util/hostname"
)

// The address of a node in the network the network keyword names is the
// first of the subnet the node was assigned, and a network giving the nodes
// no subnet of their own is refused rather than one carved out of its range.
func TestNodeIPWithNetwork(t *testing.T) {
	testhelper.Setup(t)
	conf := "[network#backend]\ntype = routed_bridge\nnetwork = 10.100.0.0/24\nips_per_node = 64\nsubnet@" + hostname.Hostname() + " = 10.100.0.64/26\n" +
		"[network#san]\ntype = lan\nnetwork = fd01::/64\nranges = fd01::5:0/120\n"
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(conf), 0600))

	r := &T{Network: "backend"}
	ip, err := r.getNodeIP(hostname.Hostname())
	require.NoError(t, err)
	assert.Equal(t, "10.100.0.65", ip.String())

	r.Network = "san"
	_, err = r.getNodeIP(hostname.Hostname())
	assert.ErrorContains(t, err, "lan network")
	b, err := os.ReadFile(rawconfig.NodeConfigFile())
	require.NoError(t, err)
	assert.Equal(t, conf, string(b), "no subnet is written")
}

// A secondary whose disk is not up to date is said not promotable, by the
// resync it waits for or by the lack of a source, and a peer disk not up to
// date is named by its peer.
func TestStatusWarnings(t *testing.T) {
	peer := func(cstate, rstate, dstate string, oos uint64) drbd.StatusConnection {
		return drbd.StatusConnection{
			Name:            "n1",
			ConnectionState: cstate,
			PeerDevices: []drbd.StatusConnPeerDevice{
				{ReplicationState: rstate, PeerDiskState: dstate, OutOfSync: oos},
			},
		}
	}
	cases := map[string]struct {
		st       drbd.Status
		expected []string
	}{
		"up to date": {
			st: drbd.Status{
				Role:        "Secondary",
				Devices:     []drbd.StatusDevice{{DiskState: "UpToDate"}},
				Connections: []drbd.StatusConnection{peer("Connected", "Established", "UpToDate", 0)},
			},
		},
		"initial sync target": {
			st: drbd.Status{
				Role:        "Secondary",
				Devices:     []drbd.StatusDevice{{DiskState: "Inconsistent"}},
				Connections: []drbd.StatusConnection{peer("Connected", "SyncTarget", "UpToDate", 2048)},
			},
			expected: []string{
				"drbd r local disk Inconsistent, not promotable before the resync ends: 2048 KiB out of sync (peers: n1 Connected SyncTarget UpToDate)",
			},
		},
		"initial sync source": {
			st: drbd.Status{
				Role:        "Primary",
				Devices:     []drbd.StatusDevice{{DiskState: "UpToDate"}},
				Connections: []drbd.StatusConnection{peer("Connected", "SyncSource", "Inconsistent", 2048)},
			},
			expected: []string{
				"drbd r peer n1 volume 0 disk Inconsistent",
			},
		},
		"sync source lost": {
			st: drbd.Status{
				Role:        "Secondary",
				Devices:     []drbd.StatusDevice{{DiskState: "Inconsistent"}},
				Connections: []drbd.StatusConnection{peer("Connecting", "Off", "DUnknown", 0)},
			},
			expected: []string{
				"drbd r local disk Inconsistent, not promotable: no connected peer holds up to date data to resync from (peers: n1 Connecting Off DUnknown)",
				"drbd r peer n1 volume 0 disk DUnknown",
			},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.expected, statusWarnings("r", c.st))
		})
	}
}
