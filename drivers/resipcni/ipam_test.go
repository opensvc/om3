//go:build linux

package resipcni

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStaticIPAMTellsThePluginTheAddress pins the rewrite that moves the
// addressing from the plugin to om: the plugin keeps the wiring and the
// routes of the network, and is told which address to configure.
func TestStaticIPAMTellsThePluginTheAddress(t *testing.T) {
	conf := []byte(`{
	  "cniVersion": "0.3.0",
	  "name": "backend3",
	  "type": "bridge",
	  "bridge": "obr_backend3",
	  "isGateway": true,
	  "ipMasq": false,
	  "ipam": {
	    "type": "host-local",
	    "routes": [{"dst": "0.0.0.0/0"}, {"dst": "10.100.0.0/22", "gw": "10.100.0.1"}],
	    "subnet": "10.100.0.0/24"
	  }
	}`)
	_, rng, err := net.ParseCIDR("10.100.0.0/24")
	require.NoError(t, err)

	b, err := staticIPAM(conf, net.ParseIP("10.100.0.24"), rng, net.ParseIP("10.100.0.1"))
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))

	// What the plugin does is untouched.
	assert.Equal(t, "bridge", m["type"])
	assert.Equal(t, "obr_backend3", m["bridge"])
	assert.Equal(t, true, m["isGateway"])

	ipam, ok := m["ipam"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "static", ipam["type"], "om picks the address, not the plugin")
	assert.Nil(t, ipam["subnet"], "a range to pick from is not the plugin's business any more")

	addresses, ok := ipam["addresses"].([]any)
	require.True(t, ok)
	require.Len(t, addresses, 1)
	address := addresses[0].(map[string]any)
	assert.Equal(t, "10.100.0.24/24", address["address"], "the mask is the range's")
	assert.Equal(t, "10.100.0.1", address["gateway"])

	// The routes of the network are carried over, or the container loses its
	// default route.
	routes, ok := ipam["routes"].([]any)
	require.True(t, ok)
	require.Len(t, routes, 2)
	assert.Equal(t, "0.0.0.0/0", routes[0].(map[string]any)["dst"])
	assert.Equal(t, "10.100.0.0/22", routes[1].(map[string]any)["dst"])
	assert.Equal(t, "10.100.0.1", routes[1].(map[string]any)["gw"])
}

// TestStaticIPAMOfAConfWithNoRoutes pins that a configuration naming no route
// is rewritten all the same.
func TestStaticIPAMOfAConfWithNoRoutes(t *testing.T) {
	_, rng, err := net.ParseCIDR("fdfe::/114")
	require.NoError(t, err)
	b, err := staticIPAM([]byte(`{"name": "backend1", "type": "bridge"}`), net.ParseIP("fdfe::12"), rng, net.ParseIP("fdfe::1"))
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	ipam := m["ipam"].(map[string]any)
	assert.Nil(t, ipam["routes"])
	address := ipam["addresses"].([]any)[0].(map[string]any)
	assert.Equal(t, "fdfe::12/114", address["address"])
	assert.Equal(t, "fdfe::1", address["gateway"])
}

func TestStaticIPAMRefusesAConfItCannotRead(t *testing.T) {
	_, rng, _ := net.ParseCIDR("10.100.0.0/24")
	_, err := staticIPAM([]byte("not json"), net.ParseIP("10.100.0.24"), rng, net.ParseIP("10.100.0.1"))
	require.Error(t, err)
}
