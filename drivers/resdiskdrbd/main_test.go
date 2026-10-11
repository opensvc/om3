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
