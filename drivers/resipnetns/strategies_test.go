//go:build linux

package resipnetns_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/hostname"

	_ "github.com/opensvc/om3/v3/drivers/rescontainerdocker"
	_ "github.com/opensvc/om3/v3/drivers/resipnetns"
)

// alertsOfKinds returns the alerts of the kinds of the validation of the
// configuration of a svc, as "<level> <key>: <comment>".
func alertsOfKinds(t *testing.T, conf string, kinds ...string) []string {
	t.Helper()
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "web"}
	o, err := object.NewSvc(p, object.WithConfigData([]byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"+conf)), object.WithVolatile(true))
	require.NoError(t, err)
	alerts, err := o.Config().Validate()
	require.NoError(t, err)
	l := make([]string, 0)
	for _, a := range alerts {
		m := a.Unstructured()
		if !slices.Contains(kinds, m["kind"].(string)) {
			continue
		}
		l = append(l, m["level"].(string)+" "+m["key"].(string)+": "+m["comment"].(string))
	}
	return l
}

// A static address of an ip.netns names the device of the node it is
// plumbed on, or the network saying it, as a layer 2 network does, which has
// no address to draw: naming an address and a network is one way, not two,
// and is not warned of. An address named and one om drew still are two.
func TestValidateAddressing(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	require.NoError(t, capabilities.Scan(t.Context()), "a section of a driver this node can not use is not validated further")
	const head = "[container#0]\ntype = docker\nimage = ghcr.io/opensvc/pause\n[ip#1]\ntype = netns\nnetns = container#0\n"
	alerts := func(conf string) []string {
		return alertsOfKinds(t, head+conf, "missing strategy", "missing keyword", "conflicting strategies")
	}

	assert.Equal(t,
		[]string{"error ip#1.: no addressing strategy is followed: set name (static), or network or addr (ipam)"},
		alerts(""))
	assert.Equal(t,
		[]string{"error ip#1.dev: dev is not set, but the static addressing strategy requires it"},
		alerts("name = 10.99.0.5\nnetmask = 24\n"))
	assert.Empty(t, alerts("name = 10.99.0.5\nnetmask = 24\ndev = br0\n"), "a static address on a device")
	assert.Empty(t, alerts("name = 10.99.0.5\nnetmask = 24\nnetwork = seg\n"), "a static address on the device of a network")
	assert.Empty(t, alerts("network = seg\n"), "a network alone")
	assert.Equal(t,
		[]string{"warning ip#1.addr: name and addr are both set, but name is the static addressing strategy and addr the ipam one"},
		alerts("name = 10.99.0.5\nnetmask = 24\nnetwork = seg\naddr = 10.99.0.6\n"))
	assert.Equal(t,
		[]string{"error ip#1.dev: dev is not set, but the static addressing strategy requires it"},
		alerts("name = 10.99.0.5\nnetmask = 24\nnetwork =\n"), "an empty network says no device")
}
