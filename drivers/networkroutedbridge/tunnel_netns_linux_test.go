package networkroutedbridge

import (
	"net"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/plog"
)

// logNoder is a noder giving the network a logger, all the tunnel functions
// need.
type logNoder struct{}

func (logNoder) MergedConfig() *xconfig.T { return nil }
func (logNoder) Log() *plog.Logger        { return plog.NewDefaultLogger() }
func (logNoder) Nodes() ([]string, error) { return nil, nil }

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
		// The thread is left locked: it ends with the goroutine, and with
		// it the namespace.
		runtime.LockOSThread()
		ns, err := netns.New()
		if !assert.NoError(t, err) {
			return
		}
		defer ns.Close()
		fn(t)
	}()
	<-done
}

func addrDummy(t *testing.T, cidrs ...string) {
	t.Helper()
	la := netlink.NewLinkAttrs()
	la.Name = "under0"
	require.NoError(t, netlink.LinkAdd(&netlink.Dummy{LinkAttrs: la}))
	link, err := netlink.LinkByName("under0")
	require.NoError(t, err)
	for _, s := range cidrs {
		addr, err := netlink.ParseAddr(s)
		require.NoError(t, err)
		require.NoError(t, netlink.AddrAdd(link, addr))
	}
	require.NoError(t, netlink.LinkSetUp(link))
}

func endpoints(t *testing.T, name string) (net.IP, net.IP) {
	t.Helper()
	link, err := netlink.LinkByName(name)
	require.NoError(t, err)
	switch tun := link.(type) {
	case *netlink.Iptun:
		return tun.Local, tun.Remote
	case *netlink.Ip6tnl:
		return tun.Local, tun.Remote
	case *netlink.Gretun:
		return tun.Local, tun.Remote
	}
	t.Fatalf("%s is a %s, no tunnel", name, link.Type())
	return nil, nil
}

// TestModTunnelChangesTheEndpoints pins that a tunnel whose endpoints
// changed is modified, for each mode: the handle test was inverted, which
// left the tunnel as it was while the setup said it modified it, and the gre
// one was rebuilt as an ip6tnl.
func TestModTunnelChangesTheEndpoints(t *testing.T) {
	for _, tc := range []struct {
		mode, local, peer, newPeer string
	}{
		{"ipip", "10.9.0.1", "10.9.0.2", "10.9.0.3"},
		{"ip6ip6", "fd00::1", "fd00::2", "fd00::3"},
		{"gre", "10.9.0.1", "10.9.0.2", "10.9.0.3"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			inNetns(t, func(t *testing.T) {
				addrDummy(t, "10.9.0.1/24", "fd00::1/64")
				r := New()
				r.SetNoder(logNoder{})
				r.SetName("test")
				local, peer, newPeer := net.ParseIP(tc.local), net.ParseIP(tc.peer), net.ParseIP(tc.newPeer)
				name := tunName(local, peer)
				if err := r.addTunnel(name, tc.mode, local, peer); err != nil {
					t.Skipf("no %s tunnel on this kernel: %s", tc.mode, err)
				}
				l, p := endpoints(t, name)
				require.True(t, l.Equal(local))
				require.True(t, p.Equal(peer))

				link, err := netlink.LinkByName(name)
				require.NoError(t, err)
				assert.False(t, r.isSameTunnelEndpoints(link, local, newPeer))
				require.NoError(t, r.modTunnel(name, tc.mode, local, newPeer))
				l, p = endpoints(t, name)
				assert.True(t, l.Equal(local), "local %s", l)
				assert.True(t, p.Equal(newPeer), "remote %s, want %s", p, newPeer)

				found, err := r.getTunnelByEndpoints(local, newPeer)
				require.NoError(t, err)
				if assert.NotNil(t, found, "found by its endpoints") {
					assert.Equal(t, name, found.Attrs().Name)
				}
			})
		})
	}
}

// TestPeerIPPrefersTheGatewayOfThePeer pins the v2 meaning of gateway@node:
// the address this node reaches the peer at, the gateway of the routes to
// its subnet and the remote endpoint of the tunnel to it, before the address
// of the node, its addr or its name resolved.
func TestPeerIPPrefersTheGatewayOfThePeer(t *testing.T) {
	_, ipnet, err := net.ParseCIDR("10.100.0.0/16")
	require.NoError(t, err)
	nodeIP := func() (net.IP, error) { return net.ParseIP("10.9.0.2"), nil }

	ip, err := peerIP("n2", "10.9.0.254", "ip4", ipnet, nodeIP)
	require.NoError(t, err)
	assert.Equal(t, "10.9.0.254", ip.String(), "the gateway of the peer first")

	ip, err = peerIP("n2", "", "ip4", ipnet, nodeIP)
	require.NoError(t, err)
	assert.Equal(t, "10.9.0.2", ip.String(), "the address of the node without a gateway")

	_, err = peerIP("n2", "10.100.0.1", "ip4", ipnet, nodeIP)
	assert.ErrorContains(t, err, "in the network 10.100.0.0/16 itself")

	_, err = peerIP("n2", "no-such-host.invalid", "ip4", ipnet, nodeIP)
	assert.ErrorContains(t, err, "gateway@n2")
}
