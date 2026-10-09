//go:build linux

package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/xconfig"
)

type fakeNetwork struct {
	network.Networker
	typ string
}

func (t fakeNetwork) Type() string        { return t.typ }
func (t fakeNetwork) IsClusterWide() bool { return t.typ == "lan" }

// fakeNetworks has san a lan network and default a bridge one.
func fakeNetworks(t *testing.T) {
	t.Helper()
	prev := lookupNetwork
	t.Cleanup(func() { lookupNetwork = prev })
	lookupNetwork = func(name string) (network.Networker, error) {
		switch name {
		case "san":
			return fakeNetwork{typ: "lan"}, nil
		case "default":
			return fakeNetwork{typ: "bridge"}, nil
		}
		return nil, nil
	}
}

// writeIP runs what a write of an object administrator goes through: the
// keyword policy over what the write changes, then the host address check.
// An empty from is an object being created.
func writeIP(t *testing.T, from, to string) error {
	t.Helper()
	p, _ := naming.ParsePath("test/svc/foo")
	var fromCfg *xconfig.T
	if from != "" {
		fromCfg = configOf(t, from)
	}
	toCfg := configOf(t, to)
	if err := configRbacChanges(admin, p.Kind, fromCfg, toCfg); err != nil {
		return err
	}
	return hostIPRbac(fromCfg, toCfg)
}

const ipNode = "[DEFAULT]\nnodes = n1 n2\n"

// An object administrator adds an ip.host drawing its address from a lan
// network, the network saying the address, the interface and the netmask.
func TestHostIPFromALANNetwork(t *testing.T) {
	knownNodes(t)
	fakeNetworks(t)
	require.NoError(t, writeIP(t, "", ipNode+"[ip#1]\ntype = host\nnetwork = san\n"))
	require.NoError(t, writeIP(t, "", ipNode+"[ip#1]\nnetwork = san\n"), "host is the default type")
	require.NoError(t, writeIP(t, "", ipNode+"[ip#1]\ntype = host\nnetwork = san\nexpose = 3260/tcp\n"))
}

// Naming what the network says is choosing an address, an interface or a
// segment of the node.
func TestHostIPChoosingWhatTheNetworkSays(t *testing.T) {
	knownNodes(t)
	fakeNetworks(t)
	for _, extra := range []string{
		"name = fd01::5:1\n",
		"dev = eth0\n",
		"netmask = 64\n",
		"addr = fd01::5:1\n",
	} {
		assert.Error(t, writeIP(t, "", ipNode+"[ip#1]\ntype = host\nnetwork = san\n"+extra), extra)
	}
	assert.Error(t, writeIP(t, "", ipNode+"[ip#1]\ntype = host\n"), "no network, no address to draw")
}

// Only a lan network is one to draw a node address from.
func TestHostIPFromAnotherNetwork(t *testing.T) {
	knownNodes(t)
	fakeNetworks(t)
	err := writeIP(t, "", ipNode+"[ip#1]\ntype = host\nnetwork = default\n")
	assert.ErrorContains(t, err, "a bridge network rather than a lan one")
	assert.Error(t, writeIP(t, "", ipNode+"[ip#1]\ntype = host\nnetwork = nosuch\n"))

	from := ipNode + "[ip#1]\ntype = host\nnetwork = san\n"
	assert.Error(t, writeIP(t, from, ipNode+"[ip#1]\ntype = host\nnetwork = default\n"), "moved to a bridge network")
}

// The addr om wrote is not the user's to set, and a section holding it stays
// the user's to edit and to remove.
func TestHostIPWithTheAddrOmWrote(t *testing.T) {
	knownNodes(t)
	fakeNetworks(t)
	from := ipNode + "[ip#1]\ntype = host\nnetwork = san\naddr = fd01::5:52\n"
	require.NoError(t, writeIP(t, from, from+"comment = portal\n"), "an edit leaves addr alone")
	require.NoError(t, writeIP(t, from, ipNode), "the section removed, the address released")
	require.NoError(t, writeIP(t, from, ipNode+"[ip#1]\ntype = host\nnetwork = san\n"), "addr unset to draw anew")
	assert.Error(t, writeIP(t, from, ipNode+"[ip#1]\ntype = host\nnetwork = san\naddr = fd01::5:53\n"), "another address chosen")
}

// An address root named, or put on an interface it chose, stays root's.
func TestHostIPRootMade(t *testing.T) {
	knownNodes(t)
	fakeNetworks(t)
	named := ipNode + "[ip#1]\ntype = host\nname = fd01::5:1\ndev = eth0\nnetwork = san\n"
	assert.Error(t, writeIP(t, named, ipNode), "removing an address root named")
	assert.Error(t, writeIP(t, named, ipNode+"[ip#1]\ntype = host\nname = fd01::5:1\ndev = eth0\nnetwork = other\n"), "moving it to another network")
}
