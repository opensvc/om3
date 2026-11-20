package networkl2bridge

import (
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/opensvc/om3/v3/core/netstatus"
	"github.com/opensvc/om3/v3/util/plog"
)

// inNetns runs fn in a new network namespace, on a thread locked to it, and
// skips the test when not root.
func inNetns(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("needs root to create a network namespace")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		runtime.LockOSThread()
		// The thread is left locked: it ends with the goroutine, and
		// with it the namespace.
		origin, err := netns.Get()
		if !assert.NoError(t, err) {
			return
		}
		defer origin.Close()
		ns, err := netns.New()
		if !assert.NoError(t, err) {
			return
		}
		defer ns.Close()
		fn(t)
	}()
	<-done
}

func link(t *testing.T, name string) netlink.Link {
	t.Helper()
	l, err := netlink.LinkByName(name)
	require.NoError(t, err, name)
	return l
}

func absent(t *testing.T, name string) {
	t.Helper()
	_, err := netlink.LinkByName(name)
	_, ok := err.(netlink.LinkNotFoundError)
	assert.True(t, ok, "%s is absent: %v", name, err)
}

func addVeth(t *testing.T, name, peer string) {
	t.Helper()
	la := netlink.NewLinkAttrs()
	la.Name = name
	require.NoError(t, netlink.LinkAdd(&netlink.Veth{LinkAttrs: la, PeerName: peer}))
}

func addDummy(t *testing.T, name string) netlink.Link {
	t.Helper()
	la := netlink.NewLinkAttrs()
	la.Name = name
	require.NoError(t, netlink.LinkAdd(&netlink.Dummy{LinkAttrs: la}))
	return link(t, name)
}

func masterOf(t *testing.T, name string) string {
	t.Helper()
	idx := link(t, name).Attrs().MasterIndex
	if idx == 0 {
		return ""
	}
	m, err := netlink.LinkByIndex(idx)
	require.NoError(t, err)
	return m.Attrs().Name
}

// ipDetails returns what ip shows of the device name in the namespace of the
// thread: sysfs shows the one it was mounted in.
func ipDetails(t *testing.T, name string) string {
	t.Helper()
	b, err := exec.Command("ip", "-d", "link", "show", "dev", name).CombinedOutput()
	require.NoError(t, err, string(b))
	return string(b)
}

// disabledIPv6 returns the disable_ipv6 setting of the device name, in the
// namespace of the thread, as /proc/sys/net shows it.
func disabledIPv6(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("/proc/sys/net/ipv6/conf/" + name + "/disable_ipv6")
	require.NoError(t, err)
	return strings.TrimSpace(string(b))
}

// TestBridgeAlone sets up a bridge with no mesh, its options, and tears it
// down: the bridge om created is deleted.
func TestBridgeAlone(t *testing.T) {
	inNetns(t, func(t *testing.T) {
		log := plog.NewDefaultLogger()
		on := true
		s := settings{name: "l2", nodename: "n1", bridge: "obr_l2", mtu: 1400, stp: &on}
		var st state
		require.NoError(t, apply(log, s, &st))
		br := link(t, "obr_l2")
		assert.True(t, st.CreatedBridge)
		assert.Equal(t, stableMAC("l2", "n1").String(), br.Attrs().HardwareAddr.String())
		assert.Equal(t, "1", disabledIPv6(t, "obr_l2"), "no address, not even a link local one")
		assert.Equal(t, 1400, br.Attrs().MTU)
		assert.Contains(t, ipDetails(t, "obr_l2"), "stp_state 1 ")
		assert.NotZero(t, br.Attrs().RawFlags&uint32(net.FlagUp))

		require.NoError(t, apply(log, s, &st), "again, nothing to do")

		require.NoError(t, teardown(log, s.bridge, &st))
		absent(t, "obr_l2")
		assert.Equal(t, state{}, st)
	})
}

// TestBridgeAdopted pins that a bridge om did not create keeps its address
// and is left at the teardown.
func TestBridgeAdopted(t *testing.T) {
	inNetns(t, func(t *testing.T) {
		log := plog.NewDefaultLogger()
		la := netlink.NewLinkAttrs()
		la.Name = "br0"
		require.NoError(t, netlink.LinkAdd(&netlink.Bridge{LinkAttrs: la}))
		mac := link(t, "br0").Attrs().HardwareAddr.String()
		s := settings{name: "l2", bridge: "br0"}
		var st state
		require.NoError(t, apply(log, s, &st))
		assert.False(t, st.CreatedBridge)
		assert.Equal(t, mac, link(t, "br0").Attrs().HardwareAddr.String())
		require.NoError(t, teardown(log, s.bridge, &st))
		link(t, "br0")

		addDummy(t, "notabr")
		require.ErrorContains(t, apply(log, settings{name: "x", bridge: "notabr"}, &st), "not a bridge")
	})
}

// TestBridgePorts meshes the bridge by ports, then a vlan of them, then
// releases them, and refuses a port holding an address or enslaved
// elsewhere.
func TestBridgePorts(t *testing.T) {
	inNetns(t, func(t *testing.T) {
		log := plog.NewDefaultLogger()
		addVeth(t, "va", "vb")
		s := settings{name: "l2", bridge: "obr_l2", ports: []string{"va"}, hairpin: true, report: &netstatus.Status{}}
		var st state
		require.NoError(t, apply(log, s, &st))
		assert.Equal(t, []netstatus.Port{{Name: "va", Dev: "va"}}, s.report.Ports)
		assert.Equal(t, "obr_l2", masterOf(t, "va"))
		assert.Contains(t, ipDetails(t, "va"), "hairpin on ")
		assert.Equal(t, []string{"va"}, st.Ports)

		// A vlan of the port instead: the port is released, the vlan
		// interface created and enslaved.
		s.vlan = 100
		require.NoError(t, apply(log, s, &st))
		assert.Equal(t, "", masterOf(t, "va"))
		assert.Equal(t, "obr_l2", masterOf(t, "va.100"))
		assert.Equal(t, "1", disabledIPv6(t, "va.100"))
		assert.Equal(t, []string{"va.100"}, st.Ports)
		assert.Equal(t, []string{"va.100"}, st.VLANs)

		// No port: the vlan interface om created is deleted.
		s.ports, s.vlan = nil, 0
		require.NoError(t, apply(log, s, &st))
		absent(t, "va.100")
		assert.Empty(t, st.Ports)
		assert.Empty(t, st.VLANs)

		// A port holding an address is refused.
		addr, err := netlink.ParseAddr("192.168.10.1/24")
		require.NoError(t, err)
		require.NoError(t, netlink.AddrAdd(link(t, "vb"), addr))
		s.ports = []string{"vb"}
		require.ErrorContains(t, apply(log, s, &st), "holds the address 192.168.10.1")
		assert.Equal(t, "", masterOf(t, "vb"))

		// A port enslaved to another bridge is refused.
		la := netlink.NewLinkAttrs()
		la.Name = "br1"
		require.NoError(t, netlink.LinkAdd(&netlink.Bridge{LinkAttrs: la}))
		require.NoError(t, netlink.LinkSetMaster(link(t, "va"), link(t, "br1")))
		s.ports = []string{"va"}
		require.ErrorContains(t, apply(log, s, &st), "enslaved to br1")

		// A missing port is an error, and is reported.
		s.ports = []string{"nope"}
		s.report = &netstatus.Status{}
		require.ErrorContains(t, apply(log, s, &st), "port nope")
		if assert.Len(t, s.report.Ports, 1) {
			assert.Equal(t, "nope", s.report.Ports[0].Name)
			assert.Contains(t, s.report.Ports[0].Error, "port nope")
		}
	})
}

// TestBridgeTeardownWaitsForTheForeignPorts pins that the teardown leaves a
// bridge om created while a device it did not enslave, as the tap of a
// guest, is still on it.
func TestBridgeTeardownWaitsForTheForeignPorts(t *testing.T) {
	inNetns(t, func(t *testing.T) {
		log := plog.NewDefaultLogger()
		addVeth(t, "va", "vb")
		s := settings{name: "l2", bridge: "obr_l2", ports: []string{"va"}}
		var st state
		require.NoError(t, apply(log, s, &st))
		tap := addDummy(t, "tap0")
		require.NoError(t, netlink.LinkSetMaster(tap, link(t, "obr_l2")))

		require.ErrorContains(t, teardown(log, s.bridge, &st), "still holds tap0, which om did not enslave")
		link(t, "obr_l2")
		assert.Equal(t, "", masterOf(t, "va"), "the ports om enslaved are released")
		assert.True(t, st.CreatedBridge)

		require.NoError(t, netlink.LinkDel(tap))
		require.NoError(t, teardown(log, s.bridge, &st))
		absent(t, "obr_l2")
	})
}

func fdbPeers(t *testing.T, name string) []string {
	t.Helper()
	l, err := fdbDsts(link(t, name))
	require.NoError(t, err)
	var peers []string
	for _, ip := range l {
		peers = append(peers, ip.String())
	}
	return peers
}

// TestBridgeVXLAN meshes the bridge by a vxlan, follows the peers, creates
// the device again when its identifier changes, and deletes it once no
// longer configured.
func TestBridgeVXLAN(t *testing.T) {
	inNetns(t, func(t *testing.T) {
		log := plog.NewDefaultLogger()
		underlay := addDummy(t, "under0")
		require.NoError(t, netlink.LinkSetMTU(underlay, 9000))
		addr, err := netlink.ParseAddr("10.9.0.1/24")
		require.NoError(t, err)
		require.NoError(t, netlink.AddrAdd(underlay, addr))
		require.NoError(t, netlink.LinkSetUp(underlay))

		s := settings{
			name:      "l2",
			bridge:    "obr_l2",
			vni:       42,
			vxlanPort: defaultVXLANPort,
			local:     net.ParseIP("10.9.0.1"),
			peers:     []net.IP{net.ParseIP("10.9.0.2"), net.ParseIP("10.9.0.3")},
			peerNodes: []string{"n2", "n3"},
			report:    &netstatus.Status{},
		}
		var st state
		require.NoError(t, apply(log, s, &st))
		if assert.Len(t, s.report.Peers, 2) {
			p := s.report.Peers[1]
			assert.Equal(t, "n3", p.Nodename)
			assert.Equal(t, "10.9.0.3", p.Remote.String())
			assert.Equal(t, "10.9.0.1", p.Local.String())
			assert.Equal(t, "vx_l2", p.Tunnel)
			assert.Equal(t, netstatus.ViaVXLAN, p.Via)
			assert.Empty(t, p.Error)
		}
		s.report = nil
		vx, ok := link(t, "vx_l2").(*netlink.Vxlan)
		require.True(t, ok)
		assert.Equal(t, 42, vx.VxlanId)
		assert.Equal(t, "obr_l2", masterOf(t, "vx_l2"))
		assert.Equal(t, 8950, vx.MTU, "the mtu of the underlay less the vxlan header")
		assert.Equal(t, 8950, link(t, "obr_l2").Attrs().MTU)
		assert.ElementsMatch(t, []string{"10.9.0.2", "10.9.0.3"}, fdbPeers(t, "vx_l2"))
		assert.Equal(t, "vx_l2", st.VXLAN)
		assert.Equal(t, "1", disabledIPv6(t, "vx_l2"))

		// A node leaves.
		s.peers = s.peers[:1]
		s.peerNodes = s.peerNodes[:1]
		require.NoError(t, apply(log, s, &st))
		assert.ElementsMatch(t, []string{"10.9.0.2"}, fdbPeers(t, "vx_l2"))

		// The identifier changes: the device is created again.
		s.vni = 43
		require.NoError(t, apply(log, s, &st))
		vx, ok = link(t, "vx_l2").(*netlink.Vxlan)
		require.True(t, ok)
		assert.Equal(t, 43, vx.VxlanId)
		assert.ElementsMatch(t, []string{"10.9.0.2"}, fdbPeers(t, "vx_l2"))

		// Both meshes are refused: a loop.
		s.ports = []string{"under0"}
		require.ErrorContains(t, apply(log, s, &st), "set one")
		s.ports = nil

		// A local address the node does not hold is refused.
		s.local = net.ParseIP("10.9.0.9")
		require.ErrorContains(t, apply(log, s, &st), "not an address of this node")

		// No vxlan: the device is deleted.
		s.vni, s.local, s.peers = 0, nil, nil
		require.NoError(t, apply(log, s, &st))
		absent(t, "vx_l2")
		assert.Equal(t, "", st.VXLAN)

		require.NoError(t, teardown(log, s.bridge, &st))
		absent(t, "obr_l2")
	})
}
