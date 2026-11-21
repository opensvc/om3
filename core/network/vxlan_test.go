package network_test

import (
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/object"
	_ "github.com/opensvc/om3/v3/drivers/networkl2bridge"
	"github.com/opensvc/om3/v3/testhelper"
)

type vxlaner interface {
	VXLAN() (int, net.IP, []net.IP)
}

// newL2Network returns the l2_bridge network l2 of a cluster configuration
// ending with conf.
func newL2Network(t *testing.T, conf string) vxlaner {
	t.Helper()
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	c, err := object.NewCluster()
	require.NoError(t, err)
	b, err := os.ReadFile(c.ConfigFile())
	require.NoError(t, err)
	b = append(b, []byte("\n[network#l2]\ntype = l2_bridge\n"+conf)...)
	require.NoError(t, os.WriteFile(c.ConfigFile(), b, 0o600))
	_, err = object.SetClusterConfig()
	require.NoError(t, err)
	n, err := object.NewNode()
	require.NoError(t, err)
	nw, ok := network.NewFromNoder("l2", n).(vxlaner)
	require.True(t, ok, "a l2_bridge reports its vxlan")
	return nw
}

// A vxlan the configuration asks for, but whose address of this node does
// not resolve, is reported as its port with no address, for the firewall to
// drop all of it: the setup fails before the device, which the previous
// setup made and left up, and reporting no vxlan removed the rule dropping
// the vxlan of any host but the nodes.
func TestVXLANThatCannotBeDescribedIsDropped(t *testing.T) {
	port, local, peers := newL2Network(t, "vxlan_vni = 1001\nvxlan_port = 4790\nvxlan_addr = no-such-host.invalid\n").VXLAN()
	assert.Equal(t, 4790, port)
	assert.Nil(t, local)
	assert.Empty(t, peers)
}

// A l2_bridge with no vxlan reports none, whatever else of its
// configuration is wrong.
func TestVXLANNotAskedIsNone(t *testing.T) {
	port, _, _ := newL2Network(t, "mtu = bad\n").VXLAN()
	assert.Zero(t, port)
}
