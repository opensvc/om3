package networkl2bridge

import (
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVXLANNameFitsADeviceName(t *testing.T) {
	assert.Equal(t, "vx_l2", vxlanName("l2"))
	long := vxlanName("a-rather-long-network")
	assert.Less(t, len(long), maxDevNameLen)
	assert.True(t, strings.HasPrefix(long, "vx_"))
	assert.Equal(t, long, vxlanName("a-rather-long-network"), "stable")
	assert.NotEqual(t, long, vxlanName("a-rather-long-networl"))
}

// TestStableMACIsLocallyAdministeredUnicast pins that the address derived for
// a bridge om creates can not collide with a vendor's, nor be a multicast
// one, which a bridge refuses, nor be the one of the bridge of another node.
func TestStableMACIsLocallyAdministeredUnicast(t *testing.T) {
	for _, name := range []string{"l2", "backend", "x"} {
		mac := stableMAC(name, "n1")
		assert.Len(t, mac, 6)
		assert.Equal(t, byte(0x02), mac[0]&0x03, name)
		assert.Equal(t, mac.String(), stableMAC(name, "n1").String(), "stable")
	}
	assert.NotEqual(t, stableMAC("l2", "n1").String(), stableMAC("l3", "n1").String())
	assert.NotEqual(t, stableMAC("l2", "n1").String(), stableMAC("l2", "n2").String(), "the bridges of a mesh are on one segment")
}

func TestBridgeTypeArgs(t *testing.T) {
	assert.Nil(t, bridgeTypeArgs(settings{bridge: "obr_l2"}), "no option set, no command")
	on, off := true, false
	ageing := 5 * time.Minute
	assert.Equal(t,
		[]string{"link", "set", "dev", "obr_l2", "type", "bridge", "stp_state", "1", "mcast_snooping", "0", "ageing_time", "30000"},
		bridgeTypeArgs(settings{bridge: "obr_l2", stp: &on, mcastSnooping: &off, ageing: &ageing}),
	)
}

func TestFDBDiff(t *testing.T) {
	ip := net.ParseIP
	add, del := fdbDiff(
		[]net.IP{ip("10.0.0.2"), ip("10.0.0.4")},
		[]net.IP{ip("10.0.0.2"), ip("10.0.0.3")},
	)
	assert.Equal(t, []net.IP{ip("10.0.0.3")}, add)
	assert.Equal(t, []net.IP{ip("10.0.0.4")}, del)
}

func TestStateRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "network", "l2.l2_bridge.json")
	st, err := loadState(p)
	require.NoError(t, err, "absent is no error")
	assert.Equal(t, state{}, st)
	st.CreatedBridge = true
	st.addPort("eth1")
	st.addPort("eth1")
	st.addVLAN("eth2.100")
	st.VXLAN = "vx_l2"
	require.NoError(t, st.save(p))
	got, err := loadState(p)
	require.NoError(t, err)
	assert.Equal(t, state{CreatedBridge: true, Ports: []string{"eth1"}, VLANs: []string{"eth2.100"}, VXLAN: "vx_l2"}, got)
	require.NoError(t, removeState(p))
	require.NoError(t, removeState(p), "absent is no error")
}
