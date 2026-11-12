package clusterhb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"

	_ "github.com/opensvc/om3/v3/core/clusterhb"
)

func clusterAlerts(t *testing.T, conf string) []string {
	t.Helper()
	testhelper.Setup(t)
	c, err := object.NewCluster(object.WithConfigData([]byte(conf)))
	require.NoError(t, err)
	alerts, err := c.Config().Validate()
	require.NoError(t, err)
	l := make([]string, 0)
	for _, a := range alerts.Errors() {
		l = append(l, a.Key.String()+": "+a.Comment)
	}
	return l
}

const clusterConf = `
[cluster]
nodes = n1 n2
`

func TestClusterConfigMulticastGroup(t *testing.T) {
	for name, tc := range map[string]struct {
		conf string
		want []string
	}{
		"an IPv4 group needs no intf": {
			conf: `
[hb#v4]
type = multicast
addr = 224.3.29.71
`,
			want: []string{},
		},
		"the default group needs no intf": {
			conf: `
[hb#v4]
type = multicast
`,
			want: []string{},
		},
		"an IPv6 group without intf is refused": {
			conf: `
[hb#v6]
type = multicast
addr = ff02::1:4141
`,
			want: []string{"hb#v6.intf: hb#v6: the IPv6 group ff02::1:4141 needs intf, on nodes n1, n2"},
		},
		"an IPv6 group not link-local needs intf too": {
			conf: `
[hb#v6]
type = multicast
addr = ff05::4141
`,
			want: []string{"hb#v6.intf: hb#v6: the IPv6 group ff05::4141 needs intf, on nodes n1, n2"},
		},
		"an IPv6 group with intf on all nodes is accepted": {
			conf: `
[hb#v6]
type = multicast
addr = ff02::1:4141
intf = eth0
`,
			want: []string{},
		},
		"an IPv6 group with intf scoped on all nodes is accepted": {
			conf: `
[hb#v6]
type = multicast
addr = ff02::1:4141
intf@n1 = eth0
intf@n2 = br0
`,
			want: []string{},
		},
		"an IPv6 group with intf scoped on one node is refused on the other": {
			conf: `
[hb#v6]
type = multicast
addr = ff02::1:4141
intf@n1 = eth0
`,
			want: []string{"hb#v6.intf: hb#v6: the IPv6 group ff02::1:4141 needs intf, on node n2"},
		},
		"an IPv6 group scoped on one node needs intf on that node only": {
			conf: `
[hb#v6]
type = multicast
addr = 224.3.29.71
addr@n2 = ff02::1:4141
`,
			want: []string{"hb#v6.intf: hb#v6: the IPv6 group ff02::1:4141 needs intf, on node n2"},
		},
		"a zone in the group is refused": {
			conf: `
[hb#v6]
type = multicast
addr = ff02::1:4141%eth0
`,
			want: []string{"hb#v6.addr: hb#v6: the group ff02::1:4141%eth0 names a zone: name the interface with intf, on nodes n1, n2"},
		},
		"a unicast address as group is refused": {
			conf: `
[hb#v4]
type = multicast
addr = 10.0.0.1
`,
			want: []string{"hb#v4.addr: hb#v4: 10.0.0.1 is not a multicast group, on nodes n1, n2"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, clusterAlerts(t, clusterConf+tc.conf))
		})
	}
}

func TestClusterConfigUnicastAddr(t *testing.T) {
	for name, tc := range map[string]struct {
		conf string
		want []string
	}{
		"IPv6 addresses are accepted": {
			conf: `
[hb#1]
type = unicast
addr@n1 = fd01::11
addr@n2 = FD01:0:0:0::12
`,
			want: []string{},
		},
		"names and no addr are accepted": {
			conf: `
[hb#1]
type = unicast
addr@n1 = n1.example.com
`,
			want: []string{},
		},
		"a zoned address is refused": {
			conf: `
[hb#1]
type = unicast
addr@n1 = fd01::11
addr@n2 = fe80::12%eth0
`,
			want: []string{"hb#1.addr: hb#1: the address fe80::12%eth0 names a zone: IPv6 link-local addresses are not supported, use a global or unique local address, on node n2"},
		},
		"a link-local address is refused": {
			conf: `
[hb#1]
type = unicast
addr@n1 = fe80::11
addr@n2 = fd01::12
`,
			want: []string{"hb#1.addr: hb#1: the address fe80::11 is link-local: IPv6 link-local addresses are not supported, use a global or unique local address, on node n1"},
		},
		"the nodes of the heartbeat are checked": {
			conf: `
[hb#1]
type = unicast
nodes = n1
addr@n1 = fd01::11
addr@n2 = fe80::12%eth0
`,
			want: []string{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, clusterAlerts(t, clusterConf+tc.conf))
		})
	}
}

// Two unicast heartbeats listening on the same port of an address both listen
// on, on a node of both, are refused: the rx of the second could not bind,
// and its tx was seen beating, its frames received by the rx of the first.
func TestClusterConfigUnicastSamePort(t *testing.T) {
	for name, tc := range map[string]struct {
		conf string
		want []string
	}{
		"the default port twice": {
			conf: `
[hb#1]
type = unicast

[hb#u5]
type = unicast
`,
			want: []string{"hb#u5.port: hb#u5: the port 10000 is the one hb#1 listens on, on nodes n1, n2: use another port, or another addr on each node"},
		},
		"another port": {
			conf: `
[hb#1]
type = unicast

[hb#u5]
type = unicast
port = 10001
`,
			want: []string{},
		},
		"the same port on another address of each node": {
			conf: `
[hb#1]
type = unicast
addr@n1 = 10.0.0.1
addr@n2 = 10.0.0.2

[hb#u5]
type = unicast
addr@n1 = 10.0.1.1
addr@n2 = 10.0.1.2
`,
			want: []string{},
		},
		"the same address written otherwise on one node": {
			conf: `
[hb#1]
type = unicast
addr@n1 = fd01::12
addr@n2 = 10.0.0.2

[hb#u5]
type = unicast
addr@n1 = FD01:0:0:0::12
addr@n2 = 10.0.1.2
`,
			want: []string{"hb#u5.port: hb#u5: the port 10000 is the one hb#1 listens on, on node n1: use another port, or another addr on each node"},
		},
		"an address and every address": {
			conf: `
[hb#1]
type = unicast
addr = 0.0.0.0

[hb#u5]
type = unicast
addr@n1 = 10.0.1.1
addr@n2 = 10.0.1.2
`,
			want: []string{"hb#u5.port: hb#u5: the port 10000 is the one hb#1 listens on, on nodes n1, n2: use another port, or another addr on each node"},
		},
		"heartbeats sharing no node": {
			conf: `
[hb#1]
type = unicast
nodes = n1

[hb#u5]
type = unicast
nodes = n2
`,
			want: []string{},
		},
		"a multicast heartbeat on the port": {
			conf: `
[hb#1]
type = unicast

[hb#m]
type = multicast
port = 10000
`,
			want: []string{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, clusterAlerts(t, clusterConf+tc.conf))
		})
	}
}
