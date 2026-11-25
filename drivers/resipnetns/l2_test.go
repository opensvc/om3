//go:build linux

package resipnetns

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/golang-collections/collections/set"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/drivers/resip"
)

// fakeL2 is a layer 2 network named seg, whose bridge is obr_seg: it has no
// address, and om allocates none in it.
type fakeL2 struct {
	network.Networker
}

func (fakeL2) Name() string               { return "seg" }
func (fakeL2) BackendDevName() string     { return "obr_seg" }
func (fakeL2) IsLayer2() bool             { return true }
func (fakeL2) IPNet() (*net.IPNet, error) { return nil, errors.New("no address") }

// fakeNS is a network namespace entered by doing nothing: what is done in it
// is done where the test runs.
type fakeNS struct {
	ns.NetNS
}

func (fakeNS) Do(f func(ns.NetNS) error) error { return f(nil) }

var testObjectID = uuid.MustParse("6f2b0b0e-58c7-4a51-9d0c-0d5b1f6f1e11")

// newL2Resource returns the resource rid of the object obj, in the namespace
// of container#0, naming the fake layer 2 network.
func newL2Resource(t *testing.T, rid string, obj resource.ObjectDriver) *T {
	t.Helper()
	r := &T{NetNS: "container#0", Network: "seg", ObjectID: testObjectID, CheckCarrier: true}
	r.Tags = set.New()
	id, err := resourceid.Parse(rid)
	require.NoError(t, err)
	r.ResourceID = id
	r._alloc = resip.ResolvedAllocation(fakeL2{})
	if obj != nil {
		r.SetObject(obj)
	}
	return r
}

// The hardware address of an interface on a layer 2 network is a locally
// administered unicast one, the same whenever it is derived from the same
// object and resource, and another for another resource, object or node.
func TestStableMAC(t *testing.T) {
	mac := stableMAC("6f2b0b0e-58c7-4a51-9d0c-0d5b1f6f1e11", "ip#1", "")
	require.Len(t, mac, 6)
	assert.Equal(t, byte(0x02), mac[0]&0x03, "locally administered, unicast: %s", mac)
	assert.Equal(t, mac, stableMAC("6f2b0b0e-58c7-4a51-9d0c-0d5b1f6f1e11", "ip#1", ""), "stable")
	assert.NotEqual(t, mac, stableMAC("6f2b0b0e-58c7-4a51-9d0c-0d5b1f6f1e11", "ip#2", ""), "another resource")
	assert.NotEqual(t, mac, stableMAC("7f2b0b0e-58c7-4a51-9d0c-0d5b1f6f1e11", "ip#1", ""), "another object")
	assert.NotEqual(t, mac, stableMAC("6f2b0b0e-58c7-4a51-9d0c-0d5b1f6f1e11", "ip#1", "n1"), "another node")
}

// The macaddr keyword wins over the stable address, which only a layer 2
// network gives: the interface of another network keeps the address the
// kernel chose. The instances of a flex object have one each, the topology
// being the one the manifest context gave the resource.
func TestMacAddr(t *testing.T) {
	r := newL2Resource(t, "ip#1", &fakeObject{})
	assert.Equal(t, stableMAC(testObjectID.String(), "ip#1", "").String(), r.macAddr())

	r.MacAddr = "ce:32:cc:ca:41:33"
	assert.Equal(t, "ce:32:cc:ca:41:33", r.macAddr(), "an explicit address wins")

	other := newRouteTestResource(t, "ip#1", "container#0", gatewayNone)
	other.SetObject(&fakeObject{})
	other._alloc = resip.ResolvedAllocation(nil)
	assert.Empty(t, other.macAddr(), "no network: the address of the kernel is left")

	flex := newL2Resource(t, "ip#1", &fakeObject{})
	flex.Topology = topology.Flex
	assert.NotEqual(t, stableMAC(testObjectID.String(), "ip#1", "").String(), flex.macAddr(), "an instance of a flex object has its own")
	flex.Shared = true
	assert.Equal(t, stableMAC(testObjectID.String(), "ip#1", "").String(), flex.macAddr(), "a shared resource is one for all the instances")
}

// A layer 2 network says the bridge, and the bridge mode. With no name, the
// resource holds no address, and what needs one is refused. With a name, the
// netmask is required, the network having none. Publish is refused either
// way, the node reaching no address of the segment.
func TestConfigureL2(t *testing.T) {
	for _, tc := range []struct {
		name    string
		set     func(r *T)
		wantErr string
		check   func(t *testing.T, r *T)
	}{
		{
			name: "layer 2 only: the bridge and the mode of the network",
			set:  func(r *T) {},
			check: func(t *testing.T, r *T) {
				assert.Equal(t, "obr_seg", r.Dev)
				assert.Equal(t, "bridge", r.Mode)
				assert.True(t, r.l2Only())
				assert.True(t, r.setsNoDefaultRoute())
				assert.Nil(t, r.ipaddr())
			},
		},
		{
			name: "layer 2 only: the bridge named as dev is accepted",
			set:  func(r *T) { r.Dev, r.Mode = "obr_seg", "bridge" },
		},
		{
			name:    "layer 2 only: expose needs an address",
			set:     func(r *T) { r.Expose = []string{"80/tcp"} },
			wantErr: "expose needs an address",
		},
		{
			name: "layer 2 only: expose none asks for nothing",
			set:  func(r *T) { r.Expose = []string{"none"} },
		},
		{
			name:    "layer 2 only: gateway needs an address",
			set:     func(r *T) { r.Gateway = "10.99.0.1" },
			wantErr: "gateway needs an address",
		},
		{
			name:    "layer 2 only: publish is refused",
			set:     func(r *T) { r.Publish = []string{"80/tcp"} },
			wantErr: "publish is not supported on network seg",
		},
		{
			name:    "another mode is refused",
			set:     func(r *T) { r.Mode = "macvlan" },
			wantErr: "mode macvlan is not supported on network seg",
		},
		{
			name:    "the dedicated tag is refused",
			set:     func(r *T) { r.Tags.Insert(tagDedicated) },
			wantErr: "mode dedicated is not supported on network seg",
		},
		{
			name:    "another dev is refused",
			set:     func(r *T) { r.Dev = "br0" },
			wantErr: "dev br0 is not the bridge of network seg, obr_seg",
		},
		{
			name:    "a macaddr that is no hardware address is refused",
			set:     func(r *T) { r.MacAddr = "nope" },
			wantErr: "macaddr",
		},
		{
			name:    "static: the netmask is required",
			set:     func(r *T) { r.Name = "10.99.0.5" },
			wantErr: "netmask is not set",
		},
		{
			name: "static: no gateway, no default route",
			set:  func(r *T) { r.Name, r.Netmask = "10.99.0.5", "24" },
			check: func(t *testing.T, r *T) {
				assert.Equal(t, "obr_seg", r.Dev)
				assert.False(t, r.l2Only())
				assert.True(t, r.setsNoDefaultRoute())
				assert.True(t, r.sourceRouteSkipped())
				assert.Empty(t, r.Gateway, "none is taken from the network")
				assert.Equal(t, "10.99.0.5", r.ipaddr().String())
			},
		},
		{
			name: "static: the gateway of the configuration sets the default route",
			set:  func(r *T) { r.Name, r.Netmask, r.Gateway = "10.99.0.5", "24", "10.99.0.1" },
			check: func(t *testing.T, r *T) {
				assert.False(t, r.setsNoDefaultRoute())
				assert.False(t, r.sourceRouteSkipped())
				assert.Equal(t, gatewayOwn, r.gatewayRank)
				assert.Equal(t, []string{"default", "via", "10.99.0.1"}, defaultRouteToSet("", r.Gateway, "eth0"))
			},
		},
		{
			name:    "static: publish is refused",
			set:     func(r *T) { r.Name, r.Netmask, r.Publish = "10.99.0.5", "24", []string{"80/tcp"} },
			wantErr: "publish is not supported on network seg",
		},
		{
			name: "static: expose is accepted",
			set:  func(r *T) { r.Name, r.Netmask, r.Expose = "10.99.0.5", "24", []string{"80/tcp"} },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The namespace is not running: it holds nothing.
			r := newL2Resource(t, "ip#1", nsObject{netns: fakeNetNS{}})
			tc.set(r)
			err := r.Configure()
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				assert.ErrorIs(t, r.ConfigError(context.Background()), err, "its ConfigError while it holds nothing")
				return
			}
			require.NoError(t, err)
			assert.NoError(t, r.netErr)
			assert.NoError(t, r.startRefusal())
			if tc.check != nil {
				tc.check(t, r)
			}
		})
	}
}

// A resource on a layer 2 network with no gateway sets no default route, so
// it is not the one the other resources of the namespace leave the route to:
// the namespace would have none. With a gateway of its own, it prevails as
// any other.
func TestRouteOwnerOnL2(t *testing.T) {
	obj := &fakeObject{}
	l2 := newL2Resource(t, "ip#0", nil)
	l2.Name, l2.Netmask = "10.99.0.5", "24"
	l2._ipaddr = net.ParseIP("10.99.0.5")
	other := newRouteTestResource(t, "ip#1", "container#0", gatewayNone)
	other._ipaddr = net.ParseIP("10.0.0.5")
	other._alloc = resip.ResolvedAllocation(nil)
	obj.resources = resource.Drivers{l2, other}
	l2.SetObject(obj)
	other.SetObject(obj)
	require.NoError(t, l2.Configure())
	ctx := context.Background()
	assert.Equal(t, "ip#1", other.routeOwner(ctx).RID(), "the lower resource id on the segment sets none")

	only := newL2Resource(t, "ip#0", nil)
	obj.resources = resource.Drivers{only, other}
	only.SetObject(obj)
	require.NoError(t, only.Configure())
	assert.Equal(t, "ip#1", other.routeOwner(ctx).RID(), "a resource with no address sets none")

	l2.Gateway = "10.99.0.1"
	obj.resources = resource.Drivers{l2, other}
	require.NoError(t, l2.Configure())
	assert.Equal(t, "ip#0", other.routeOwner(ctx).RID(), "a gateway of its own prevails")
}

// The node has no address on a layer 2 segment: the ping a start is aborted
// by, and the one a provision is refused by, would ask another network, and
// are not sent.
func TestL2SendsNoPingFromTheNode(t *testing.T) {
	r := newL2Resource(t, "ip#1", &fakeObject{})
	r.Name, r.Netmask = "127.0.0.1", "8"
	require.NoError(t, r.Configure())
	ctx := context.Background()
	assert.False(t, r.Abort(ctx), "an address answering the node does not abort")
	assert.Empty(t, r.ProvisionConflicts(ctx))
}

// An address another host of the segment answers for is refused, unless the
// answer is the one of the interface itself, or the resource is tagged
// nonrouted. A probe failing refuses nothing.
func TestCheckL2Duplicate(t *testing.T) {
	prev := arpProbe
	defer func() { arpProbe = prev }()
	r := newL2Resource(t, "ip#1", &fakeObject{})
	r.Name, r.Netmask = "10.99.0.5", "24"
	require.NoError(t, r.Configure())
	self := fakeNS{}
	answer := func(mac string, err error) {
		arpProbe = func(ip net.IP, dev string) (net.HardwareAddr, error) {
			assert.Equal(t, "10.99.0.5", ip.String())
			assert.Equal(t, "eth0", dev)
			if mac == "" {
				return nil, err
			}
			hw, _ := net.ParseMAC(mac)
			return hw, err
		}
	}
	answer("", nil)
	assert.NoError(t, r.checkL2Duplicate(self, "eth0"), "no answer")
	answer("52:54:00:00:00:01", nil)
	assert.ErrorContains(t, r.checkL2Duplicate(self, "eth0"), "10.99.0.5 is already held on the segment of network seg, by 52:54:00:00:00:01")
	answer(r.macAddr(), nil)
	assert.NoError(t, r.checkL2Duplicate(self, "eth0"), "its own answer")
	answer("", errors.New("no socket"))
	assert.NoError(t, r.checkL2Duplicate(self, "eth0"), "a probe failing refuses nothing")
	answer("52:54:00:00:00:01", nil)
	r.Tags.Insert(tagNonRouted)
	assert.NoError(t, r.checkL2Duplicate(self, "eth0"), "nonrouted")
}

// An arp probe is a request with no sender address, and its answer a reply
// sent from the address probed: a request, or a reply of another address, is
// none.
func TestARPProbePacket(t *testing.T) {
	hw, _ := net.ParseMAC("02:00:00:00:00:01")
	ip := net.ParseIP("10.99.0.5")
	probe := arpProbePacket(hw, ip)
	require.Len(t, probe, arpLen)
	assert.Equal(t, []byte{0, 1, 8, 0, 6, 4, 0, 1}, probe[:8], "an ipv4 over ethernet request")
	assert.Equal(t, []byte(hw), probe[8:14])
	assert.Equal(t, []byte{0, 0, 0, 0}, probe[14:18], "no sender address")
	assert.Equal(t, []byte{10, 99, 0, 5}, probe[24:28])
	assert.Nil(t, arpProbeAnswer(probe, ip), "a request")

	other, _ := net.ParseMAC("52:54:00:00:00:02")
	reply := make([]byte, arpLen)
	copy(reply, probe)
	reply[7] = arpReply
	copy(reply[8:14], other)
	copy(reply[14:18], ip.To4())
	assert.Equal(t, other.String(), arpProbeAnswer(reply, ip).String())
	assert.Nil(t, arpProbeAnswer(reply, net.ParseIP("10.99.0.6")), "a reply of another address")
	assert.Nil(t, arpProbeAnswer(reply[:20], ip), "a short packet")
}
