package network_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/object"
	_ "github.com/opensvc/om3/v3/drivers/networkbridge"
	_ "github.com/opensvc/om3/v3/drivers/networklo"
	"github.com/opensvc/om3/v3/testhelper"
)

// A setup of some networks, as the daemon makes of the one whose configuration
// changed, applies the firewall rules of every network: the ruleset is
// rewritten whole, and rendered from the networks set up only, it dropped the
// rules of the others.
func TestSetupOfOneNetworkKeepsTheFirewallRulesOfAll(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	c, err := object.NewCluster()
	require.NoError(t, err)
	b, err := os.ReadFile(c.ConfigFile())
	require.NoError(t, err)
	b = append(b, []byte("\n[network#br1]\ntype = bridge\nnetwork = 10.50.0.0/16\n\n[network#br2]\ntype = bridge\nnetwork = 10.51.0.0/16\n")...)
	require.NoError(t, os.WriteFile(c.ConfigFile(), b, 0o600))

	var got []string
	restore := network.CaptureFirewall(func(nws []network.Networker) {
		got = make([]string, 0, len(nws))
		for _, nw := range nws {
			got = append(got, nw.Name())
		}
	})
	defer restore()

	n, err := object.NewNode()
	require.NoError(t, err)
	// A name no network has: the setup touches no network of the node, and
	// still applies the rules of all.
	require.NoError(t, network.Setup(n, "nope"))
	assert.ElementsMatch(t, []string{"br1", "br2", "lo", "default"}, got)
}
