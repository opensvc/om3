package rescontainerocibase

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/drivers/rescontainer"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// netnsAddr is an ip resource configuring an address of a network
	// naming nameservers in the network namespace of a container.
	netnsAddr struct {
		resource.Driver
		rid         string
		netns       string
		nameservers []string
	}

	// resourcesObject is an object of the resources given.
	resourcesObject struct {
		resources resource.Drivers
	}
)

func (t netnsAddr) RID() string                  { return t.rid }
func (t netnsAddr) NetNSResource() string        { return t.netns }
func (t netnsAddr) NetworkNameservers() []string { return t.nameservers }

func (t resourcesObject) Log() *plog.Logger                                       { return plog.NewDefaultLogger() }
func (t resourcesObject) VarDir() string                                          { return "" }
func (t resourcesObject) ResourceByID(string) resource.Driver                     { return nil }
func (t resourcesObject) ResourcesByDrivergroups([]driver.Group) resource.Drivers { return nil }
func (t resourcesObject) Resources() resource.Drivers                             { return t.resources }

func addrs(l ...string) []netip.Addr {
	out := make([]netip.Addr, len(l))
	for i, s := range l {
		out[i] = netip.MustParseAddr(s)
	}
	return out
}

// The nameservers the network filter of a container opens are the ones om
// writes in its resolv.conf, by the same precedence, and the ones of the
// node when om writes none: a container asking the nameservers of its
// network has the nodes serving the cluster dns left closed.
func TestNetNameservers(t *testing.T) {
	saved := nodeNameservers
	t.Cleanup(func() { nodeNameservers = saved })
	nodeNameservers = func() []netip.Addr { return addrs("127.0.0.53") }

	newBT := func(t *testing.T, dns, dnsExtra []string, resources ...resource.Driver) *BT {
		bt := &BT{DNS: dns, DNSExtra: dnsExtra, ObjectDomain: "ns1.svc.c1"}
		require.NoError(t, bt.SetRID("container#1"))
		bt.SetObject(resourcesObject{resources: resources})
		return bt
	}
	cluster := []string{"10.29.0.11", "10.29.0.12", "10.29.0.13"}

	t.Run("none written", func(t *testing.T) {
		bt := newBT(t, nil, nil)
		require.True(t, bt.resolvConf().IsZero())
		assert.Equal(t, addrs("127.0.0.53"), bt.netNameservers(), "the resolver of the engine is the one of the node")
	})

	t.Run("cluster.dns", func(t *testing.T) {
		bt := newBT(t, cluster, nil)
		assert.Equal(t, addrs(cluster...), bt.netNameservers())
	})

	t.Run("network.dns", func(t *testing.T) {
		bt := newBT(t, cluster, nil,
			netnsAddr{rid: "ip#1", netns: "container#1", nameservers: []string{"192.168.100.53"}},
			netnsAddr{rid: "ip#2", netns: "container#2", nameservers: []string{"192.168.200.53"}},
		)
		assert.Equal(t, addrs("192.168.100.53"), bt.netNameservers(),
			"the nameservers of the network replace the ones of the cluster, which stay closed")
	})

	t.Run("network.dns of the container whose netns is joined", func(t *testing.T) {
		bt := newBT(t, cluster, nil,
			netnsAddr{rid: "ip#2", netns: "container#2", nameservers: []string{"192.168.200.53"}},
		)
		bt.NetNS = "container#2"
		assert.Equal(t, addrs("192.168.200.53"), bt.netNameservers())
	})

	t.Run("dns extras", func(t *testing.T) {
		bt := newBT(t, nil, []string{"9.9.9.9", "2620:fe::fe"})
		assert.Equal(t, addrs("9.9.9.9", "2620:fe::fe"), bt.netNameservers(), "no nameserver of the node: om wrote a resolver")

		bt = newBT(t, cluster[:1], []string{"9.9.9.9"})
		assert.Equal(t, addrs("10.29.0.11", "9.9.9.9"), bt.netNameservers())

		bt = newBT(t, nil, []string{"9.9.9.9"},
			netnsAddr{rid: "ip#1", netns: "container#1", nameservers: []string{"192.168.100.53"}})
		assert.Equal(t, addrs("192.168.100.53", "9.9.9.9"), bt.netNameservers())
	})

	t.Run("no more than a resolver reads", func(t *testing.T) {
		bt := newBT(t, cluster, []string{"9.9.9.9"})
		assert.Equal(t, addrs(cluster...), bt.netNameservers(), "the nameserver not written in the file is not opened")
	})

	t.Run("the file and the filter agree", func(t *testing.T) {
		bt := newBT(t, cluster, []string{"9.9.9.9"},
			netnsAddr{rid: "ip#1", netns: "container#1", nameservers: []string{"192.168.100.53"}})
		var written []string
		ea := &ExecutorArg{BT: bt, WriteResolvConf: func(resolvConf rescontainer.ResolvConf) (string, error) {
			written = append(written, resolvConf.String())
			return "/var/lib/opensvc/resolv.conf", nil
		}}
		_, err := ea.resolvConfMount()
		require.NoError(t, err)
		require.Len(t, written, 1)
		opened := bt.netNameservers()
		assert.Equal(t, addrs("192.168.100.53", "9.9.9.9"), opened)
		for _, addr := range opened {
			assert.Contains(t, written[0], "nameserver "+addr.String()+"\n")
		}
		assert.Equal(t, len(opened), strings.Count(written[0], "nameserver "))
	})

	t.Run("the target of the filter", func(t *testing.T) {
		bt := newBT(t, cluster[:1], nil)
		target := bt.netTarget()
		require.NotNil(t, target.DNS)
		assert.Equal(t, addrs("10.29.0.11"), target.DNS())
	})
}
