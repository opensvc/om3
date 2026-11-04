package resipcni

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

// The routes a network namespace holds already are not asked of the plugin
// again: the address added first keeps the default route.
func TestTheRoutesHeldAreNotAskedAgain(t *testing.T) {
	conf := []byte(`{"name":"default","type":"bridge","ipam":{"type":"static","routes":[{"dst":"0.0.0.0/0"},{"dst":"10.22.0.0/16","gw":"10.22.0.1"}]}}`)

	b, dropped, err := dropHeldRoutes(conf, map[string]bool{"0.0.0.0/0": true, "10.137.0.0/16": true})
	require.NoError(t, err)
	assert.Equal(t, []string{"0.0.0.0/0"}, dropped)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	routes := m["ipam"].(map[string]any)["routes"].([]any)
	require.Len(t, routes, 1)
	assert.Equal(t, "10.22.0.0/16", routes[0].(map[string]any)["dst"])

	b, dropped, err = dropHeldRoutes(conf, map[string]bool{"::/0": true})
	require.NoError(t, err)
	assert.Empty(t, dropped)
	assert.Equal(t, conf, b, "nothing held: the configuration as it was")
}

func TestTheDestinationOfADefaultRouteIsItsFamilyAny(t *testing.T) {
	assert.Equal(t, "0.0.0.0/0", routeDst(netlink.Route{Family: netlink.FAMILY_V4}))
	assert.Equal(t, "::/0", routeDst(netlink.Route{Family: netlink.FAMILY_V6}))
	assert.Equal(t, "::/0", routeDst(netlink.Route{Gw: net.ParseIP("fdfe::1")}))
	_, dst, _ := net.ParseCIDR("10.137.0.0/16")
	assert.Equal(t, "10.137.0.0/16", routeDst(netlink.Route{Dst: dst}))
}
