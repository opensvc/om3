package resiphost_test

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

	_ "github.com/opensvc/om3/v3/drivers/resiphost"
)

// strategyAlerts returns the conflicting strategies alerts of the validation
// of the configuration of a svc, as "<level> <key>: <comment>".
func strategyAlerts(t *testing.T, conf string) []string {
	t.Helper()
	return alertsOfKinds(t, conf, "conflicting strategies")
}

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

// An ip addressed both statically and from a network is warned of, once per
// pair of keywords, whatever the scope or the alias they are written by. One
// way alone is not, nor the netmask of a static address overriding the one of
// a network, which the ipam strategy does not exclude.
func TestValidateConflictingStrategies(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	require.NoError(t, capabilities.Scan(t.Context()), "a section of a driver this node can not use is not validated further")

	assert.Equal(t,
		[]string{"warning ip#1.network: name and network are both set, but name is the static addressing strategy and network the ipam one"},
		strategyAlerts(t, "[ip#1]\ntype = host\nname = 1.2.3.4\nnetwork = default\n"))

	assert.Empty(t, strategyAlerts(t, "[ip#1]\ntype = host\nname = 1.2.3.4\n"), "name alone")
	assert.Empty(t, strategyAlerts(t, "[ip#1]\ntype = host\nnetwork = default\n"), "network alone")
	assert.Empty(t, strategyAlerts(t, "[ip#1]\ntype = host\nnetwork = default\nnetmask = 24\n"), "a netmask overriding the one of the network")
	assert.Empty(t, strategyAlerts(t, "[DEFAULT]\nname = 1.2.3.4\n[ip#1]\ntype = host\nnetwork = default\n"), "a value inherited from DEFAULT")

	l := strategyAlerts(t, "[ip#1]\nipname@"+hostname.Hostname()+" = 1.2.3.4\nnetwork = default\naddr = 1.2.3.5\n")
	assert.ElementsMatch(t, []string{
		"warning ip#1.network: ipname and network are both set, but ipname is the static addressing strategy and network the ipam one",
		"warning ip#1.addr: ipname and addr are both set, but ipname is the static addressing strategy and addr the ipam one",
	}, l, "the default driver of the section, a scoped alias")
}

// An ip naming neither an address nor a network to draw one from follows no
// addressing strategy, and a static address naming no device of the node
// misses what its strategy requires: no start can build either, so the
// validation refuses them. A network says the device, and a value inherited
// from DEFAULT counts as set.
func TestValidateRequiredAddressing(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	require.NoError(t, capabilities.Scan(t.Context()))
	missing := func(conf string) []string {
		return alertsOfKinds(t, conf, "missing strategy", "missing keyword")
	}

	assert.Equal(t,
		[]string{"error ip#1.: no addressing strategy is followed: set name (static), or network or addr (ipam)"},
		missing("[ip#1]\ntype = host\n"))
	assert.Equal(t,
		[]string{"error ip#1.dev: dev is not set, but the static addressing strategy requires it"},
		missing("[ip#1]\ntype = host\nname = 1.2.3.4\n"))

	assert.Empty(t, missing("[ip#1]\ntype = host\nname = 1.2.3.4\ndev = lo\n"), "a static address on a device")
	assert.Empty(t, missing("[ip#1]\ntype = host\nnetwork = default\n"), "an address drawn from a network")
	assert.Empty(t, missing("[ip#1]\ntype = host\nipname@"+hostname.Hostname()+" = 1.2.3.4\nipdev = lo\n"), "a scoped alias")
	assert.Empty(t, missing("[DEFAULT]\ndev = lo\n[ip#1]\ntype = host\nname = 1.2.3.4\n"), "a device inherited from DEFAULT")
	assert.Empty(t, missing("[DEFAULT]\nipdev@"+hostname.Hostname()+" = lo\n[ip#1]\ntype = host\nname = 1.2.3.4\n"), "a device inherited from DEFAULT by a scoped alias")

	// A keyword written with an empty value sets nothing, as the field a
	// designer adds for a strategy picked, to be filled in.
	assert.Equal(t,
		[]string{"error ip#1.: no addressing strategy is followed: set name (static), or network or addr (ipam)"},
		missing("[ip#1]\ntype = host\nnetwork =\n"), "an empty network")
	assert.Equal(t,
		[]string{"error ip#1.dev: dev is not set, but the static addressing strategy requires it"},
		missing("[ip#1]\ntype = host\nname = 1.2.3.4\ndev =  \n"), "an empty dev")
	assert.Equal(t,
		[]string{"error ip#1.dev: dev is not set, but the static addressing strategy requires it"},
		missing("[DEFAULT]\ndev =\n[ip#1]\ntype = host\nname = 1.2.3.4\n"), "an empty dev in DEFAULT")
	assert.Empty(t, missing("[ip#1]\ntype = host\nname =\nnetwork = default\n"), "an empty name follows no static strategy")
	assert.Empty(t, strategyAlerts(t, "[ip#1]\ntype = host\nname =\nnetwork = default\n"), "an empty name conflicts with no network")
}
